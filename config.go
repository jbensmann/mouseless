package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Action string

const (
	ActionTapHold            Action = "tap-hold"
	ActionTapHoldNext        Action = "tap-hold-next"
	ActionTapHoldNextRelease Action = "tap-hold-next-release"
	ActionMulti              Action = "multi"
	ActionLayer              Action = "layer"
	ActionToggleLayer        Action = "toggle-layer"
	ActionReloadConfig       Action = "reload-config"
	ActionMove               Action = "move"
	ActionScroll             Action = "scroll"
	ActionSpeed              Action = "speed"
	ActionButton             Action = "button"
	ActionExec               Action = "exec"
)

// RawConfig defines the structure of the config file.
type RawConfig struct {
	Devices           []string   `yaml:"devices"`
	StartCommand      string     `yaml:"startCommand"`
	BaseMouseSpeed    float64    `yaml:"baseMouseSpeed"`
	MouseStartSpeed   float64    `yaml:"mouseStartSpeed"`
	MouseAcceleration float64    `yaml:"mouseAcceleration"`
	MouseDeceleration float64    `yaml:"mouseDeceleration"`
	BaseScrollSpeed   float64    `yaml:"baseScrollSpeed"`
	Layers            []RawLayer `yaml:"layers"`
}

type RawLayer struct {
	Name        string            `yaml:"name"`
	PassThrough *bool             `yaml:"passThrough"`
	Bindings    map[string]string `yaml:"bindings"`
}

// Config is the parsed form of RawConfig.
type Config struct {
	Devices           []string
	StartCommand      string
	BaseMouseSpeed    float64
	MouseAcceleration float64
	MouseDeceleration float64
	MouseStartSpeed   float64
	BaseScrollSpeed   float64
	Layers            []*Layer
}

type Layer struct {
	Name        string
	PassThrough bool // default true
	Bindings    map[uint16]Binding
}

type Binding interface {
	binding()
}

type BaseBinding struct {
}

func (b BaseBinding) binding() {}

type MultiBinding struct {
	BaseBinding
	Binding1 Binding
	Binding2 Binding
}

type TapHoldBinding struct {
	BaseBinding
	TapBinding       Binding
	HoldBinding      Binding
	TimeoutMs        int64
	TapOnNext        bool
	TapOnNextRelease bool
}

type LayerBinding struct {
	BaseBinding
	Layer string
}
type ToggleLayerBinding struct {
	BaseBinding
	Layer string
}
type ReloadConfigBinding struct {
	BaseBinding
}
type KeyBinding struct {
	BaseBinding
	KeyCombo []uint16
}
type MoveBinding struct {
	BaseBinding
	X, Y float64
}
type ScrollBinding struct {
	BaseBinding
	X, Y float64
}
type SpeedBinding struct {
	BaseBinding
	Speed float64
}
type ButtonBinding struct {
	BaseBinding
	Button MouseButton
}
type ExecBinding struct {
	BaseBinding
	Command string
}

// readConfig reads and parses the configuration from the given file.
func readConfig(fileName string) (*Config, error) {
	rawConfig, err := readRawConfig(fileName)
	if err != nil {
		return nil, err
	}

	config := Config{
		MouseAcceleration: math.Inf(1),
		MouseDeceleration: math.Inf(1),
	}
	config.Devices = rawConfig.Devices
	config.StartCommand = rawConfig.StartCommand
	config.BaseMouseSpeed = rawConfig.BaseMouseSpeed
	if rawConfig.MouseAcceleration > 0 {
		config.MouseAcceleration = rawConfig.MouseAcceleration
	}
	if rawConfig.MouseDeceleration > 0 {
		config.MouseDeceleration = rawConfig.MouseDeceleration
	}
	config.MouseStartSpeed = rawConfig.MouseStartSpeed
	config.BaseScrollSpeed = rawConfig.BaseScrollSpeed

	for i, l := range rawConfig.Layers {
		layer, err := parseLayer(l)
		if err != nil {
			return nil, fmt.Errorf("failed to parse layer %v : %v", i, err)
		}
		config.Layers = append(config.Layers, layer)
	}

	log.Debugf("config: %+v", config)
	return &config, nil
}

// readRawConfig reads the configuration from the given file.
func readRawConfig(fileName string) (*RawConfig, error) {
	var rawConfig RawConfig

	file, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(file, &rawConfig)
	if err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// parseLayer parses a single RawLayer to Layer.
func parseLayer(rawLayer RawLayer) (*Layer, error) {
	var layer Layer

	if rawLayer.Name == "" {
		return nil, fmt.Errorf("no name given")
	}

	layer.Name = rawLayer.Name
	layer.Bindings = make(map[uint16]Binding)
	if rawLayer.PassThrough == nil {
		layer.PassThrough = true
	} else {
		layer.PassThrough = *rawLayer.PassThrough
	}

	if rawLayer.Bindings == nil {
		rawLayer.Bindings = make(map[string]string)
	}
	for key, bind := range rawLayer.Bindings {
		code, err := parseKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the key '%v': %v", key, err)
		}
		binding, err := parseBinding(bind)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the binding '%v': %v", bind, err)
		}
		layer.Bindings[code] = binding
	}

	return &layer, nil
}

// parseBinding parses a single binding of a layer.
func parseBinding(rawBinding string) (binding Binding, err error) {
	spaceSplit := strings.Fields(rawBinding)
	action := strings.TrimSpace(spaceSplit[0])
	argString := strings.TrimSpace(strings.Replace(rawBinding, action, "", 1))

	var args []string
	if len(spaceSplit) > 0 {
		for _, s := range spaceSplit[1:] {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			args = append(args, s)
		}
	}

	switch action {
	case string(ActionMulti):
		metaArgs := strings.Split(argString, ";")
		if len(metaArgs) != 2 {
			return nil, fmt.Errorf("action requires exactly two meta arguments (separated by ;)")
		}
		b1, err := parseBinding(metaArgs[0])
		if err != nil {
			return nil, err
		}
		b2, err := parseBinding(metaArgs[1])
		if err != nil {
			return nil, err
		}
		binding = MultiBinding{Binding1: b1, Binding2: b2}
	case string(ActionTapHold):
		tapHoldBinding, err := parseTapHoldBinding(argString)
		if err != nil {
			return nil, err
		}
		tapHoldBinding.TapOnNext = false
		binding = tapHoldBinding
	case string(ActionTapHoldNext):
		tapHoldBinding, err := parseTapHoldBinding(argString)
		if err != nil {
			return nil, err
		}
		tapHoldBinding.TapOnNext = true
		binding = tapHoldBinding
	case string(ActionTapHoldNextRelease):
		tapHoldBinding, err := parseTapHoldBinding(argString)
		if err != nil {
			return nil, err
		}
		tapHoldBinding.TapOnNextRelease = true
		binding = tapHoldBinding
	case string(ActionLayer):
		if len(args) != 1 {
			return nil, fmt.Errorf("action requires exactly one argument")
		}
		binding = LayerBinding{Layer: args[0]}
	case string(ActionToggleLayer):
		if len(args) != 1 {
			return nil, fmt.Errorf("action requires exactly one argument")
		}
		binding = ToggleLayerBinding{Layer: args[0]}
	case string(ActionReloadConfig):
		if len(args) != 0 {
			return nil, fmt.Errorf("action requires zero arguments")
		}
		binding = ReloadConfigBinding{}
	case string(ActionMove):
		if len(args) != 2 {
			return nil, fmt.Errorf("action requires exactly two arguments")
		}
		x, y := 0.0, 0.0
		if x, err = strconv.ParseFloat(args[0], 64); err != nil {
			return nil, fmt.Errorf("first argument must be a number")
		}
		if y, err = strconv.ParseFloat(args[1], 64); err != nil {
			return nil, fmt.Errorf("second argument must be a number")
		}
		binding = MoveBinding{X: x, Y: y}
	case string(ActionScroll):
		if len(args) != 1 {
			return nil, fmt.Errorf("action requires exactly one argument")
		}
		x, y := 0.0, 0.0
		switch args[0] {
		case "up":
			y = -1
		case "down":
			y = +1
		case "left":
			x = -1
		case "right":
			x = +1
		default:
			return nil, fmt.Errorf("first argument must one of up, down, left or right")
		}
		binding = ScrollBinding{X: x, Y: y}
	case string(ActionSpeed):
		if len(args) != 1 {
			return nil, fmt.Errorf("action requires exactly one argument")
		}
		speed := 0.0
		if speed, err = strconv.ParseFloat(args[0], 64); err != nil {
			return nil, fmt.Errorf("first argument must be a number")
		}
		binding = SpeedBinding{Speed: speed}
	case string(ActionButton):
		if len(args) != 1 {
			return nil, fmt.Errorf("action requires exactly one argument")
		}
		button := MouseButton(strings.ToLower(args[0]))
		if button != ButtonLeft && button != ButtonMiddle && button != ButtonRight {
			return nil, fmt.Errorf("unknown button '%v'", args[0])
		}
		binding = ButtonBinding{Button: button}
	case string(ActionExec):
		if len(args) == 0 {
			return nil, fmt.Errorf("action requires at least one argument")
		}
		binding = ExecBinding{Command: argString}
	default:
		combo, err := parseKeyCombo(rawBinding)
		if err != nil {
			return nil, fmt.Errorf("neither a valid action nor a valid key sequence")
		}
		binding = KeyBinding{KeyCombo: combo}
	}

	return binding, nil
}

func parseTapHoldBinding(argString string) (TapHoldBinding, error) {
	b := TapHoldBinding{}
	metaArgs := strings.Split(argString, ";")
	if len(metaArgs) != 3 {
		return b, fmt.Errorf("action requires exactly 3 meta arguments (separated by ;)")
	}
	b1, err := parseBinding(metaArgs[0])
	if err != nil {
		return b, err
	}
	b.TapBinding = b1
	b2, err := parseBinding(metaArgs[1])
	if err != nil {
		return b, err
	}
	b.HoldBinding = b2
	var timeout int64
	timeoutStr := strings.TrimSpace(metaArgs[2])
	if timeout, err = strconv.ParseInt(timeoutStr, 10, 64); err != nil {
		return b, fmt.Errorf("third argument must be a number: %s", timeoutStr)
	}
	b.TimeoutMs = timeout
	return b, nil
}

// parseKeyCombo parses a key combination of the form key1+key2+...
func parseKeyCombo(rawCombo string) (combo []uint16, err error) {
	for _, key := range strings.Split(rawCombo, "+") {
		code, err := parseKey(key)
		if err != nil {
			return combo, err
		}
		combo = append(combo, code)
	}
	return combo, nil
}

// parseKey parses a single key, which can be either the code itself or an alias.
func parseKey(key string) (code uint16, err error) {
	key = strings.TrimSpace(key)

	if code, ok := keyAliases[key]; ok {
		return code, nil
	}

	if code, err := strconv.Atoi(key); err == nil {
		return uint16(code), nil
	}

	return 0, fmt.Errorf("neither an integer nor a key alias")
}
