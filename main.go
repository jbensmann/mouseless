package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jbensmann/mouseless/actions"
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/handlers"
	"github.com/jbensmann/mouseless/keyboard"
	"github.com/jbensmann/mouseless/virtual"

	"github.com/fsnotify/fsnotify"
	evdev "github.com/gvalkov/golang-evdev"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

const (
	defaultConfigFile = ".config/mouseless/config.yaml"
)

var (
	version string // set during build

	configFile           string
	configDevices        []string
	configDevicesExclude []string

	keyboardDevices []*keyboard.Device
	virtualMouse    *virtual.Mouse
	virtualKeyboard *virtual.Keyboard

	keyEventChannel     chan keyboard.Event
	firstEventHandler   handlers.EventHandler
	reloadConfigChannel chan struct{}
)

var opts struct {
	Version             bool   `short:"v" long:"version" description:"Show the version"`
	Debug               bool   `short:"d" long:"debug" description:"Show verbose debug information"`
	ConfigFile          string `short:"c" long:"config" description:"Specify an alternative config file"`
	ListKeyboardDevices bool   `short:"l" long:"list-devices" description:"List all detected keyboard devices"`
	ListAllDevices      bool   `short:"L" long:"list-all-devices" description:"List all detected devices"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
	if opts.Version {
		fmt.Println(version)
		os.Exit(0)
	}
	if opts.ListKeyboardDevices {
		printDevices(true)
		os.Exit(0)
	}
	if opts.ListAllDevices {
		printDevices(false)
		os.Exit(0)
	}

	// init logging
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05.000"})
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// if no config file is given, use the default one
	configFile = opts.ConfigFile
	if configFile == "" {
		u, err := user.Current()
		if err != nil {
			exitError("Failed to get the current user", err)
		}
		configFile = filepath.Join(u.HomeDir, defaultConfigFile)
	}
	log.Debugf("Using config file: %s", configFile)
	conf, err := config.ReadConfig(configFile)
	if err != nil {
		exitError("Failed to read the config file", err)
	}
	configDevices = conf.Devices
	configDevicesExclude = conf.DevicesExclude
	run(conf)
}

func run(conf *config.Config) {
	keyEventChannel = make(chan keyboard.Event, 1000)
	reloadConfigChannel = make(chan struct{}, 1)

	allDevices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		exitError("Failed to detect keyboard devices", err)
	}

	// check if another instance of mouse is already running
	for _, device := range allDevices {
		if device.Name == "mouseless keyboard" {
			exitError(
				"Found a keyboard device with name 'mouseless keyboard'"+
					", which probably means that another instance of mouseless is already running",
				nil,
			)
		}
	}

	var usedDevices []*evdev.InputDevice
	for _, device := range allDevices {
		if shallDeviceBeUsed(device) {
			usedDevices = append(usedDevices, device)
		}
	}
	if len(usedDevices) == 0 {
		log.Warnf("No keyboard devices found")
	}

	// init virtual mouse and keyboard
	virtualMouse, err = virtual.NewMouse(conf)
	if err != nil {
		exitError("Failed to init the virtual mouse", err)
	}
	defer virtualMouse.Close()

	virtualKeyboard, err = virtual.NewKeyboard()
	if err != nil {
		exitError("Failed to init the virtual keyboard", err)
	}
	defer virtualKeyboard.Close()

	for _, device := range usedDevices {
		log.Infof("Found keyboard device: %s (%s)", device.Fn, device.Name)
		log.Debugf("Device details: %s", device)
	}

	// init keyboard devices
	for _, device := range usedDevices {
		addDevice(device)
	}

	initHandlers(conf)

	if conf.StartCommand != "" {
		log.Debugf("Executing start command: %s", conf.StartCommand)
		cmd := exec.Command("sh", "-c", conf.StartCommand)
		err := cmd.Run()
		if err != nil {
			exitError("Execution of start command failed", err)
		}
	}

	err = watchForKeyboardDevices()
	if err != nil {
		exitError("Failed to watch for keyboard devices", err)
	}

	virtualMouse.StartLoop()
	mainLoop()
}

func initHandlers(conf *config.Config) {
	executor := actions.NewExecutor(conf, virtualKeyboard, virtualMouse, reloadConfigChannel)

	defaultHandler := handlers.NewDefaultHandler()
	defaultHandler.SetLayerManager(executor)
	defaultHandler.SetNextHandler(executor)

	tapHoldHandler := handlers.NewTapHoldHandler(int64(conf.QuickTapTime))
	tapHoldHandler.SetLayerManager(executor)
	tapHoldHandler.SetNextHandler(defaultHandler)

	firstEventHandler = handlers.NewComboHandler(int64(conf.ComboTime))
	firstEventHandler.SetLayerManager(executor)
	firstEventHandler.SetNextHandler(tapHoldHandler)
}

// mainLoop processes incoming keyboard events and reload config events.
func mainLoop() {
	for {
		select {
		case <-reloadConfigChannel:
			reloadConfig()
		case e := <-keyEventChannel:
			firstEventHandler.HandleEvent(handlers.EventBinding{Event: e})
		}
	}
}

// watchForKeyboardDevices starts a watcher for devices in /dev/input, and adds or removes keyboard
// devices matching the device specification in the config file.
func watchForKeyboardDevices() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	watchPath := "/dev/input"
	log.Debugf("Watching for keyboard devices in: %s", watchPath)
	err = watcher.Add(watchPath)
	if err != nil {
		_ = watcher.Close()
		return fmt.Errorf("failed to add watcher for %s: %w", watchPath, err)
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case e, ok := <-watcher.Events:
				if !ok {
					log.Errorf("Device watcher closed unexpectetly")
					return
				}
				log.Debugf("Detected a device event: %s", e)
				if strings.HasPrefix(e.Name, "/dev/input/event") {
					if e.Op.Has(fsnotify.Create) {
						deviceCreated(e)
					} else if e.Op.Has(fsnotify.Remove) {
						deviceRemoved(e)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Errorf("Device watcher closed unexpectetly")
					return
				}
				log.Warnf("Device watcher error: %v", err)
			}
		}
	}()
	return nil
}

// deviceCreated is called when a new device file is created.
func deviceCreated(e fsnotify.Event) {
	for _, dev := range keyboardDevices {
		if dev.Path() == e.Name {
			log.Infof("Device already conntected: %s", dev.Path())
			return
		}
	}
	var device *evdev.InputDevice
	var err error
	// wait for udev to fix permissions, otherwise one can get permission denied on open
	for range 3 {
		device, err = evdev.Open(e.Name)
		if err == nil {
			break
		}
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		log.Warnf("Failed to open device %s: %v", e.Name, err)
		return
	}
	if !shallDeviceBeUsed(device) {
		log.Debugf("Ignoring the device")
	} else {
		log.Infof("Detected new keyboard device: %s (%s)", device.Fn, device.Name)
		addDevice(device)
	}
}

// deviceRemoved is called when a device file is removed.
func deviceRemoved(e fsnotify.Event) {
	for i, dev := range keyboardDevices {
		if dev.Path() == e.Name {
			log.Infof("Keybord device has been removed: %s", dev)
			keyboardDevices = slices.Delete(keyboardDevices, i, i+1)
			dev.Disconnected()
			if len(keyboardDevices) == 0 {
				log.Warnf("No more keyboard devices connected to read from")
			}
			break
		}
	}
}

// addDevice adds the given keyboard device to the list of keyboard devices to read from.
func addDevice(device *evdev.InputDevice) {
	log.Infof("Reading from keyboard device: %s", device.Fn)
	kd := keyboard.NewKeyboardDevice(device, keyEventChannel)
	err := kd.GrabDevice()
	if err != nil {
		log.Warnf("Failed to grab keyboard device %s: %v", device.Fn, err)
		return
	}
	keyboardDevices = append(keyboardDevices, kd)
}

// reloadConfig reloads the config file and updates the handlers, but does not
// update the keyboard devices specification.
func reloadConfig() {
	log.Infof("Reloading the config file: %s", configFile)
	var err error
	conf, err := config.ReadConfig(configFile)
	if err != nil {
		log.Warnf("Failed to read the config file: %v", err)
		return
	}
	initHandlers(conf)
	virtualMouse.SetConfig(conf)
}

// printDevices prints all input devices with their capabilities.
func printDevices(keyboardsOnly bool) {
	devices, err := evdev.ListInputDevices("/dev/input/event*")
	if err != nil {
		exitError("Failed to list input devices", err)
	}
	headers := []string{"Name", "Device", "Keyboard", "Bus", "Vendor", "Product", "Version", "Events"}
	rows := [][]string{}
	for _, dev := range devices {
		isKeyboard := isKeyboardDevice(dev)
		if !keyboardsOnly || isKeyboard {
			keyboardText := "no"
			if isKeyboard {
				keyboardText = "yes"
			}
			var capabilities []string
			for capType := range dev.Capabilities {
				capabilities = append(capabilities, capType.Name)
			}
			sort.Strings(capabilities)
			rows = append(rows, []string{
				dev.Name,
				dev.Fn,
				keyboardText,
				fmt.Sprintf("%#04x", dev.Bustype),
				fmt.Sprintf("%#04x", dev.Vendor),
				fmt.Sprintf("%#04x", dev.Product),
				fmt.Sprintf("%#04x", dev.Version),
				strings.Join(capabilities, ","),
			})
		}
	}
	// sort by name
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})
	printTable(headers, rows)
}

// printTable prints a table with the given headers and rows.
// It dynamically calculates the width of each column.
func printTable(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, col := range row {
			if len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}
	printRow := func(row []string) {
		for i, col := range row {
			fmt.Print(col)
			if i < len(row)-1 {
				fmt.Print(strings.Repeat(" ", 2+widths[i]-len(col)))
			}
		}
		fmt.Println()
	}
	printRow(headers)
	for _, row := range rows {
		printRow(row)
	}
}

// shallDeviceBeUsed checks if the given device should be used.
// This is the case if these two conditions are met:
// 1. (config.devices is empty and device is a keyboard) or (device is listed in config.devices)
// 2. device is not listed in config.devicesExclude
func shallDeviceBeUsed(device *evdev.InputDevice) bool {
	if len(configDevices) == 0 {
		if !isKeyboardDevice(device) {
			return false
		}
	} else {
		anyMatches := false
		for _, deviceConfig := range configDevices {
			if deviceMatches(device, deviceConfig) {
				anyMatches = true
				break
			}
		}
		if !anyMatches {
			return false
		}
	}
	for _, deviceConfig := range configDevicesExclude {
		if deviceMatches(device, deviceConfig) {
			return false
		}
	}
	return true
}

// deviceMatches checks if the given device matches the given deviceConfig by checking if
// it matches either the device name or the device path.
func deviceMatches(device *evdev.InputDevice, deviceConfig string) bool {
	if deviceConfig == device.Fn || deviceConfig == device.Name {
		return true
	}
	// if it is a symlink, resolve it and check if the resolved path matches
	dest, err := filepath.EvalSymlinks(deviceConfig)
	if err == nil {
		if dest == device.Fn {
			return true
		}
	}
	return false
}

// isKeyboardDevice checks if the given device is a keyboard by checking if
// it has at least an A key or a 1 key.
func isKeyboardDevice(dev *evdev.InputDevice) bool {
	for capType, codes := range dev.Capabilities {
		if capType.Type == evdev.EV_KEY {
			for _, code := range codes {
				if code.Code == evdev.KEY_A || code.Code == evdev.KEY_KP1 {
					return true
				}
			}
		}
	}
	return false
}

// exitError logs the given error and exits the program.
func exitError(msg string, err error) {
	if err != nil {
		log.Errorf(msg+": %v", err)
	} else {
		log.Error(msg)
	}
	log.Error("Exiting")
	os.Exit(1)
}
