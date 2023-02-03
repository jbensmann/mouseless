package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

const version = "0.1.0"

const (
	mouseLoopInterval = 20 * time.Millisecond
	defaultConfigFile = ".config/mouseless/config.yaml"
)

var (
	configFile      string
	config          *Config
	keyboardDevices []*KeyboardDevice
	mouse           *VirtualMouse
	keyboard        *VirtualKeyboard
	tapHoldHandler  *TapHoldHandler

	currentLayer        *Layer
	toggleLayerKey      *uint16
	toggleLayerPrevious *Layer
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
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	listKeyboardDevices()

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
	loadConfig()

	// init virtual mouse and keyboard
	mouse, err = NewVirtualMouse()
	if err != nil {
		exitError(err, "Failed to init the virtual mouse")
	}
	defer mouse.Close()

	keyboard, err = NewVirtualKeyboard()
	if err != nil {
		exitError(err, "Failed to init the virtual keyboard")
	}
	defer keyboard.Close()

	tapHoldHandler = NewTapHoldHandler()

	// init keyboard devices
	for _, dev := range config.Devices {
		kd := NewKeyboardDevice(dev, tapHoldHandler.InChannel())
		keyboardDevices = append(keyboardDevices, kd)
		go kd.ReadLoop()
	}

	if config.StartCommand != "" {
		log.Debugf("Executing start command: %s", config.StartCommand)
		cmd := exec.Command("sh", "-c", config.StartCommand)
		err := cmd.Run()
		if err != nil {
			exitError(err, "Execution of start command failed")
		}
	}

	mainLoop()
}

func loadConfig() {
	var err error
	config, err = readConfig(configFile)
	if err != nil {
		exitError(err, "Failed to read the config file")
	}

	// set initial layer
	currentLayer = config.Layers[0]
	log.Debugf("Switching to initial layer %s", currentLayer.Name)
}

func mainLoop() {
	tapHoldHandler.StartProcessing()
	mouseTimer := time.NewTimer(math.MaxInt64)

	for {
		// check if a key was pressed
		var event *KeyboardEvent = nil
		select {
		case e := <-tapHoldHandler.OutChannel():
			event = &e
		case <-mouseTimer.C:
		}
		if event != nil {
			handleKey(event)
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

		// handle mouse movement and scrolling
		moveX := 0.0
		moveY := 0.0
		scrollX := 0.0
		scrollY := 0.0
		speedFactor := 1.0
		for code, binding := range currentLayer.Bindings {
			if tapHoldHandler.IsKeyPressed(code) {
				switch t := binding.(type) {
				case SpeedBinding:
					speedFactor *= t.Speed
				case ScrollBinding:
					scrollX += t.X
					scrollY += t.Y
				case MoveBinding:
					moveX += t.X
					moveY += t.Y
				}
			}
		}

		if moveX != 0 || moveY != 0 || scrollX != 0 || scrollY != 0 || mouse.IsMoving() {
			tickTime := mouseLoopInterval.Seconds()
			moveSpeed := config.BaseMouseSpeed * tickTime
			scrollSpeed := config.BaseScrollSpeed * tickTime
			mouseAcceleration := config.MouseAcceleration * tickTime * tickTime
			mouseDeceleration := config.MouseDeceleration * tickTime * tickTime
			mouse.Scroll(scrollX*scrollSpeed*speedFactor, scrollY*scrollSpeed*speedFactor)
			mouse.Move(moveX*moveSpeed, moveY*moveSpeed, config.MouseStartSpeed*tickTime, mouseAcceleration, mouseDeceleration, speedFactor)
			mouseTimer = time.NewTimer(mouseLoopInterval)
		} else {
			mouseTimer = time.NewTimer(math.MaxInt64)
		}
	}
}

// handleKey handles a single key event (press or release).
func handleKey(event *KeyboardEvent) {
	binding, _ := currentLayer.Bindings[event.code]

	// when no binding and pass through is enabled, insert a KeyBinding
	if binding == nil && currentLayer.PassThrough {
		binding = KeyBinding{KeyCombo: []uint16{event.code}}
	}

	// go back to the previous layer when toggleLayerKey is released
	if toggleLayerKey != nil && *toggleLayerKey == event.code && !event.isPress {
		if toggleLayerPrevious != nil {
			currentLayer = toggleLayerPrevious
			toggleLayerPrevious = nil
			toggleLayerKey = nil
			log.Debugf("Switching to layer %v", currentLayer.Name)
		}
	}

	// inform the keyboard and mouse about key releases
	if !event.isPress {
		keyboard.OriginalKeyUp(event.code)
		mouse.OriginalKeyUp(event.code)
	}

	executeBinding(event, binding)

	// switch to first layer on escape
	if event.code == evdev.KEY_ESC && event.isPress {
		currentLayer = config.Layers[0]
		log.Debugf("Switching to layer %v", currentLayer.Name)
	}
}

// executeBinding does what needs to be done for the given binding.
// For some bindings there is nothing that needs to be done, e.g. for the speed
// and move bindings.
// For tap-hold bindings, either the tap or the hold binding is executed.
func executeBinding(event *KeyboardEvent, binding interface{}) {
	log.Debugf("Executing %T: %+v", binding, binding)

	switch t := binding.(type) {
	case MultiBinding:
		executeBinding(event, t.Binding1)
		executeBinding(event, t.Binding2)
	case TapHoldBinding:
		if event.holdKey {
			executeBinding(event, t.HoldBinding)
		} else {
			executeBinding(event, t.TapBinding)
		}
	case LayerBinding:
		if event.isPress {
			// if current layer is toggled, deactivate the toggle
			if toggleLayerPrevious != nil {
				toggleLayerPrevious = nil
				toggleLayerKey = nil
			}
			for _, layer := range config.Layers {
				if layer.Name == t.Layer {
					log.Debugf("Switching to layer %v", layer.Name)
					currentLayer = layer
					break
				}
			}
		}
	case ToggleLayerBinding:
		// only allow one toggle
		if event.isPress && toggleLayerPrevious == nil {
			for _, layer := range config.Layers {
				if layer.Name == t.Layer {
					log.Debugf("Switching to layer %v", layer.Name)
					toggleLayerPrevious = currentLayer
					toggleLayerKey = &event.code
					currentLayer = layer
					break
				}
			}
		}
	case ReloadConfigBinding:
		if event.isPress {
			loadConfig()
		}
	case KeyBinding:
		if event.isPress {
			keyboard.PressKeys(event.code, t.KeyCombo)
		}
	case ButtonBinding:
		if event.isPress {
			mouse.ButtonPress(event.code, t.Button)
		}
	case ExecBinding:
		// exec
		if event.isPress {
			log.Debugf("Executing: %s", t.Command)
			cmd := exec.Command("sh", "-c", t.Command)
			err := cmd.Run()
			if err != nil {
				log.Warnf("Execution of command failed: %v", err)
			}
		}
	}
}

// listKeyboardDevices lists all available keyboard input devices.
func listKeyboardDevices() {
	devices, _ := evdev.ListInputDevices("/dev/input/by-path/*kbd*")
	devices2, _ := evdev.ListInputDevices("/dev/input/by-id/*kbd*")
	devices = append(devices, devices2...)
	log.Debugf("Available keyboard devices:")
	for _, dev := range devices {
		log.Debugf("%s %s %s\n", dev.Fn, dev.Name, dev.Phys)
	}
}

func exitError(err error, msg string) {
	if err != nil {
		log.Errorf(msg+": %v", err)
	} else {
		log.Error(msg)
	}
	os.Exit(1)
}
