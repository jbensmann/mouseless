package main

import (
	"fmt"
	"github.com/jbensmann/mouseless/actions"
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/handlers"
	"github.com/jbensmann/mouseless/keyboard"
	"github.com/jbensmann/mouseless/virtual"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

const version = "0.2.0-dev"

const (
	defaultConfigFile = ".config/mouseless/config.yaml"
)

var (
	configFile string

	keyboardDevices []*keyboard.Device
	virtualMouse    *virtual.Mouse
	virtualKeyboard *virtual.VirtualKeyboard

	eventInChannel chan keyboard.Event
	tapHoldHandler *handlers.TapHoldHandler
	comboHandler   *handlers.ComboHandler
)

var opts struct {
	Version    bool   `short:"v" long:"version" description:"Show the version"`
	Debug      bool   `short:"d" long:"debug" description:"Show verbose debug information"`
	ConfigFile string `short:"c" long:"config" description:"The config file"`
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

	eventInChannel = make(chan keyboard.Event, 1000)

	// init keyboard devices
	for _, dev := range conf.Devices {
		kd := keyboard.NewKeyboardDevice(dev, eventInChannel)
		keyboardDevices = append(keyboardDevices, kd)
		go kd.ReadLoop()
	}

	executor := actions.NewBindingExecutor(conf, virtualKeyboard, virtualMouse)

	defaultHandler := handlers.NewDefaultHandler()
	defaultHandler.SetLayerManager(executor)
	defaultHandler.SetNextHandler(executor)

	tapHoldHandler = handlers.NewTapHoldHandler(int64(conf.QuickTapTime))
	tapHoldHandler.SetLayerManager(executor)
	tapHoldHandler.SetNextHandler(defaultHandler)

	comboHandler = handlers.NewComboHandler(int64(conf.ComboTime))
	comboHandler.SetLayerManager(executor)
	comboHandler.SetNextHandler(tapHoldHandler)

	// todo: wait a second here, or maybe even earlier?

	if conf.StartCommand != "" {
		log.Debugf("Executing start command: %s", conf.StartCommand)
		cmd := exec.Command("sh", "-c", conf.StartCommand)
		err := cmd.Run()
		if err != nil {
			exitError(err, "Execution of start command failed")
		}
	}

	virtualMouse.StartLoop()
	mainLoop()
}

func mainLoop() {
	checkTimer := time.NewTimer(5 * time.Second)

	// listen for incoming keyboard events
	for {
		select {
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
	var devices []*evdev.InputDevice
	devices, _ = evdev.ListInputDevices("/dev/input/event*")

	// filter out the keyboard devices that have at least an A key or a 1 key
	var keyboardDevices []*evdev.InputDevice
	for _, dev := range devices {
		for capType, codes := range dev.Capabilities {
			if capType.Type == evdev.EV_KEY {
				for _, code := range codes {
					if code.Code == evdev.KEY_A || code.Code == evdev.KEY_KP1 {
						keyboardDevices = append(keyboardDevices, dev)
						break
					}
				}
			}
		}
	}

	// print the keyboard devices
	log.Debugf("Auto detected keyboard devices:")
	for _, dev := range keyboardDevices {
		log.Debugf("- %s: %s\n", dev.Fn, dev.Name)
	}
	return keyboardDevices
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
