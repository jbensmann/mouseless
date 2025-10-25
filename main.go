package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jbensmann/mouseless/actions"
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/handlers"
	"github.com/jbensmann/mouseless/keyboard"
	"github.com/jbensmann/mouseless/virtual"

	evdev "github.com/gvalkov/golang-evdev"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

const (
	defaultConfigFile = ".config/mouseless/config.yaml"
)

var (
	version    string
	configFile string

	keyboardDevices []*keyboard.Device
	virtualMouse    *virtual.Mouse
	virtualKeyboard *virtual.VirtualKeyboard

	eventInChannel      chan keyboard.Event
	tapHoldHandler      *handlers.TapHoldHandler
	comboHandler        *handlers.ComboHandler
	reloadConfigChannel chan struct{}
)

var opts struct {
	Version     bool   `short:"v" long:"version" description:"Show the version"`
	Debug       bool   `short:"d" long:"debug" description:"Show verbose debug information"`
	ConfigFile  string `short:"c" long:"config" description:"The config file"`
	ListDevices bool   `long:"list-devices" description:"List all found devices with their capabilities"`
}

func main() {
	var err error

	_, err = flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Println(version)
		os.Exit(0)
	}

	if opts.ListDevices {
		listAllDevices()
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
			exitError(err, "Failed to get the current user")
		}
		configFile = filepath.Join(u.HomeDir, defaultConfigFile)
	}

	log.Debugf("Using config file: %s", configFile)
	conf, err := config.ReadConfig(configFile)
	if err != nil {
		exitError(err, "Failed to read the config file")
	}
	run(conf)
}

func run(conf *config.Config) {
	eventInChannel = make(chan keyboard.Event, 1000)
	reloadConfigChannel = make(chan struct{}, 1)
	var specifiedDevices = conf.Devices

	detectedKeyboardDevices := findKeyboardDevices()

	// check if another instance of mouse is already running
	for _, device := range detectedKeyboardDevices {
		if device.Name == "mouseless" {
			exitError(nil, "Found a keyboard device with name mouseless, "+
				"which probably means that another instance of mouseless is already running")
		}
	}

	// if no devices are specified, use the detected ones
	if len(conf.Devices) == 0 {
		for _, device := range detectedKeyboardDevices {
			conf.Devices = append(conf.Devices, device.Fn)
		}
		if len(conf.Devices) == 0 {
			exitError(nil, "No keyboard devices found")
		}
	}

	// resolve device specifications to paths
	devicePaths, err := conf.GetDevicePaths()
	if err != nil {
		exitError(err, "Failed to resolve device specifications")
	}

	// init virtual mouse and keyboard
	virtualMouse, err = virtual.NewMouse(conf)
	if err != nil {
		exitError(err, "Failed to init the virtual mouse")
	}
	defer virtualMouse.Close()

	virtualKeyboard, err = virtual.NewVirtualKeyboard()
	if err != nil {
		exitError(err, "Failed to init the virtual keyboard")
	}
	defer virtualKeyboard.Close()

	// init keyboard devices
	for _, dev := range conf.Devices {
		for _, device := range detectedKeyboardDevices {
			if dev == device.Fn {
				kd := keyboard.NewKeyboardDevice(device, eventInChannel)
				keyboardDevices = append(keyboardDevices, kd)
				kd.GrabDevice()
			}
		}

	}

	initHandlers(conf)

	if conf.StartCommand != "" {
		log.Debugf("Executing start command: %s", conf.StartCommand)
		cmd := exec.Command("sh", "-c", conf.StartCommand)
		err := cmd.Run()
		if err != nil {
			exitError(err, "Execution of start command failed")
		}
	}

	go watchForKeyboardDevices(specifiedDevices)

	virtualMouse.StartLoop()
	mainLoop()
}

func initHandlers(conf *config.Config) {
	executor := actions.NewBindingExecutor(conf, virtualKeyboard, virtualMouse, reloadConfigChannel)

	defaultHandler := handlers.NewDefaultHandler()
	defaultHandler.SetLayerManager(executor)
	defaultHandler.SetNextHandler(executor)

	tapHoldHandler = handlers.NewTapHoldHandler(int64(conf.QuickTapTime))
	tapHoldHandler.SetLayerManager(executor)
	tapHoldHandler.SetNextHandler(defaultHandler)

	comboHandler = handlers.NewComboHandler(int64(conf.ComboTime))
	comboHandler.SetLayerManager(executor)
	comboHandler.SetNextHandler(tapHoldHandler)
}

func mainLoop() {
	checkTimer := time.NewTimer(5 * time.Second)

	// listen for incoming keyboard events
	for {
		select {
		case <-reloadConfigChannel:
			reloadConfig()
		case e := <-eventInChannel:
			comboHandler.HandleEvent(handlers.EventBinding{Event: e})
		case <-checkTimer.C:
		}

		// check if at least one device is opened
		oneDeviceOpen := false
		for _, device := range keyboardDevices {
			if device.IsOpen() {
				oneDeviceOpen = true
			}
		}
		if !oneDeviceOpen {
			log.Warnf("No keyboard device could be opened:")
			for i, device := range keyboardDevices {
				log.Warnf("Device %d: %s: %s", i+1, device.DeviceName(), device.LastOpenError())
			}
			time.Sleep(10 * time.Second)
		}
	}
}

// findKeyboardDevices finds all available keyboard input devices.
func findKeyboardDevices() []*evdev.InputDevice {
	devices, _ := evdev.ListInputDevices("/dev/input/event*")

	var keyboardDevices []*evdev.InputDevice
	log.Debugf("Auto detected keyboard devices:")
	for _, dev := range devices {
		if isKeyboardDevice(dev) {
			keyboardDevices = append(keyboardDevices, dev)
			log.Debugf("- %s: %s\n", dev.Fn, dev.Name)
		}
	}
	return keyboardDevices
}

// listAllDevices prints all input devices with their capabilities.
func listAllDevices() {
	devices, _ := evdev.ListInputDevices("/dev/input/event*")

	for _, dev := range devices {
		if isKeyboardDevice(dev) {
			keyboardDevices = append(keyboardDevices, dev)
		}
	}
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

// reloadConfig reloads the config file and updates the handlers.
// But it does not reload the keyboard devices to read from.
func reloadConfig() {
	log.Infof("Reloading the config file: %s", configFile)
	conf, err := config.ReadConfig(configFile)
	if err != nil {
		log.Warnf("Failed to read the config file: %v", err)
		return
	}
	initHandlers(conf)
	virtualMouse.SetConfig(conf)
}

func exitError(err error, msg string) {
	if err != nil {
		log.Errorf(msg+": %v", err)
	} else {
		log.Error(msg)
	}
	log.Error("Exiting")
	os.Exit(1)
}

func watchForKeyboardDevices(specifiedDevices []string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	err = watcher.Add("/dev/input")
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case e, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if e.Op&fsnotify.Create != fsnotify.Create {
				continue
			}

			if len(specifiedDevices) > 0 {
				matched := false
				for _, device := range specifiedDevices {
					if e.Name == device {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			} else if !strings.HasPrefix(e.Name, "/dev/input/event") {
				continue
			}

			log.Infof("New device detected: %s", e.Name)

			// Check if the device is already in the list if so
			// remove it, because you one can't be sure it's the
			// same device anyway.
			for i, dev := range keyboardDevices {
				if dev.DeviceName() == e.Name {
					log.Debugf("Remove old device %v", dev.DeviceName())
					keyboardDevices = append(keyboardDevices[:i], keyboardDevices[i+1:]...)
					break
				}
			}

			// wait for udev to fix permissions
			// otherwise you can get permission denied on Open()
			time.Sleep(1 * time.Second)

			var device *evdev.InputDevice
			device, err = evdev.Open(e.Name)
			if err != nil {
				log.Warnf("Failed to open device %s: %v", e.Name, err)
				continue
			}

			if !isKeyboardDevice(device) {
				log.Debugf("New device was not a keyboard")
				continue
			}

			log.Infof("Adding new device: %s", e.Name)
			kd := keyboard.NewKeyboardDevice(device, eventInChannel)
			keyboardDevices = append(keyboardDevices, kd)
			kd.GrabDevice()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Println("Watcher error:", err)
		}
	}
}

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
