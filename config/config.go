package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"strconv"
	"strings"
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
	ActionNop                Action = "nop"
)

// RawConfig defines the structure of the config file.
type RawConfig struct {
	Devices                []string   `yaml:"devices"`
	StartCommand           string     `yaml:"startCommand"`
	MouseLoopInterval      int64      `yaml:"mouseLoopInterval"`
	BaseMouseSpeed         float64    `yaml:"baseMouseSpeed"`
	StartMouseSpeed        float64    `yaml:"startMouseSpeed"`
	MouseAccelerationCurve float64    `yaml:"mouseAccelerationCurve"`
	MouseAccelerationTime  float64    `yaml:"mouseAccelerationTime"`
	MouseDecelerationCurve float64    `yaml:"mouseDecelerationCurve"`
	MouseDecelerationTime  float64    `yaml:"mouseDecelerationTime"`
	BaseScrollSpeed        float64    `yaml:"baseScrollSpeed"`
	QuickTapTime           float64    `yaml:"quickTapTime"`
	ComboTime              float64    `yaml:"comboTime"`
	Layers                 []RawLayer `yaml:"layers"`
}

type RawLayer struct {
	Name         string            `yaml:"name"`
	PassThrough  *bool             `yaml:"passThrough"`
	EnterCommand *string           `yaml:"enterCommand"`
	ExitCommand  *string           `yaml:"exitCommand"`
	Bindings     map[string]string `yaml:"bindings"`
}

// Config is the parsed form of RawConfig.
type Config struct {
	Devices                []string
	StartCommand           string
	MouseLoopInterval      int64
	QuickTapTime           float64
	ComboTime              float64
	BaseMouseSpeed         float64
	MouseAccelerationCurve float64
	MouseAccelerationTime  float64
	MouseDecelerationCurve float64
	MouseDecelerationTime  float64
	StartMouseSpeed        float64
	BaseScrollSpeed        float64
	Layers                 []*Layer
}

type Layer struct {
	Name            string
	PassThrough     bool // default true
	EnterCommand    *string
	ExitCommand     *string
	Bindings        map[uint16]Binding
	ComboBindings   map[uint16]map[uint16]Binding
	WildcardBinding Binding
}

type Binding interface {
	binding()
}

type BaseBinding struct {
}

func (b BaseBinding) binding() {}

type MultiBinding struct {
	BaseBinding
	Bindings []Binding
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
type NopBinding struct {
	BaseBinding
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

// ReadConfig reads and parses the configuration from the given file.
func ReadConfig(fileName string) (*Config, error) {
	// read the file
	configFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	configString, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	return ParseConfig(configString)
}

// ParseConfig parses the given configuration.
func ParseConfig(configBytes []byte) (*Config, error) {
	var rawConfig RawConfig
	err := yaml.Unmarshal(configBytes, &rawConfig)
	if err != nil {
		return nil, err
	}

	config := Config{
		MouseAccelerationCurve: 1.0,
		MouseDecelerationCurve: 1.0,
	}
	config.Devices = rawConfig.Devices
	config.StartCommand = rawConfig.StartCommand
	if rawConfig.MouseLoopInterval > 0 {
		config.MouseLoopInterval = rawConfig.MouseLoopInterval
	} else {
		config.MouseLoopInterval = 20
	}
	config.BaseMouseSpeed = rawConfig.BaseMouseSpeed
	if rawConfig.MouseAccelerationCurve > 0 {
		config.MouseAccelerationCurve = rawConfig.MouseAccelerationCurve
	}
	config.MouseAccelerationTime = rawConfig.MouseAccelerationTime
	if rawConfig.MouseDecelerationCurve > 0 {
		config.MouseDecelerationCurve = rawConfig.MouseDecelerationCurve
	}
	config.MouseDecelerationTime = rawConfig.MouseDecelerationTime
	config.StartMouseSpeed = rawConfig.StartMouseSpeed
	config.BaseScrollSpeed = rawConfig.BaseScrollSpeed
	config.QuickTapTime = rawConfig.QuickTapTime
	if rawConfig.ComboTime > 0 {
		config.ComboTime = rawConfig.ComboTime
	} else {
		config.ComboTime = 25
	}
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

// parseLayer parses a single RawLayer to Layer.
func parseLayer(rawLayer RawLayer) (*Layer, error) {
	var layer Layer

	if rawLayer.Name == "" {
		return nil, fmt.Errorf("no name given")
	}

	layer.Name = rawLayer.Name
	layer.EnterCommand = rawLayer.EnterCommand
	layer.ExitCommand = rawLayer.ExitCommand
	layer.Bindings = make(map[uint16]Binding)
	layer.ComboBindings = make(map[uint16]map[uint16]Binding)
	if rawLayer.PassThrough == nil {
		layer.PassThrough = true
	} else {
		layer.PassThrough = *rawLayer.PassThrough
	}

	if rawLayer.Bindings == nil {
		rawLayer.Bindings = make(map[string]string)
	}
	for key, bind := range rawLayer.Bindings {
		codes, err := parseKeyCombo(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the key '%v': %v", key, err)
		}
		binding, err := parseBinding(bind)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the binding '%v': %v", bind, err)
		}
		if len(codes) == 1 {
			if codes[0] == WildcardKey {
				layer.WildcardBinding = binding
			} else {
				layer.Bindings[codes[0]] = binding
			}
		} else if len(codes) == 2 {
			if _, ok := layer.ComboBindings[codes[0]]; !ok {
				layer.ComboBindings[codes[0]] = make(map[uint16]Binding)
			}
			if _, ok := layer.ComboBindings[codes[1]]; !ok {
				layer.ComboBindings[codes[1]] = make(map[uint16]Binding)
			}
			layer.ComboBindings[codes[0]][codes[1]] = binding
			layer.ComboBindings[codes[1]][codes[0]] = binding
		} else {
			return nil, fmt.Errorf("combos with more than 2 keys are not supported: '%v'", key)
		}
	}

	return &layer, nil
}

// parseBinding parses a single binding of a layer.
func parseBinding(rawBinding string) (binding Binding, err error) {
	if len(rawBinding) == 0 {
		return nil, fmt.Errorf("binding is empty")
	}
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
		if len(metaArgs) < 2 {
			return nil, fmt.Errorf("action requires at least two meta arguments (separated by ;)")
		}
		multiBinding := MultiBinding{}
		for _, arg := range metaArgs {
			b, err := parseBinding(arg)
			if err != nil {
				return nil, err
			}
			multiBinding.Bindings = append(multiBinding.Bindings, b)
		}
		binding = multiBinding
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
	case string(ActionNop):
		if len(args) != 0 {
			return nil, fmt.Errorf("action does not take any argument")
		}
		binding = NopBinding{}
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
