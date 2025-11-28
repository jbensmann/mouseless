package actions

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/handlers"
	"github.com/jbensmann/mouseless/keyboard"
	"github.com/jbensmann/mouseless/virtual"
	log "github.com/sirupsen/logrus"
)

type ExecutedBinding struct {
	cause   *keyboard.Event
	binding config.Binding
}

type Executor struct {
	config              *config.Config
	virtualKeyboard     *virtual.Keyboard
	virtualMouse        *virtual.Mouse
	reloadConfigChannel chan<- struct{}

	currentLayer *config.Layer
	// remember all keys that toggled a layer, and from which layer they came from
	toggleLayerKeys     []uint16
	toggleLayerPrevious []*config.Layer
	// remember all ExecPressReleaseBindings that have been executed
	execPressReleaseBindings map[uint16]config.ExecPressReleaseBinding
}

func NewExecutor(
	conf *config.Config,
	virtualKeyboard *virtual.Keyboard,
	virtualMouse *virtual.Mouse,
	reloadConfigChannel chan struct{},
) *Executor {
	b := Executor{
		config:                   conf,
		virtualKeyboard:          virtualKeyboard,
		virtualMouse:             virtualMouse,
		reloadConfigChannel:      reloadConfigChannel,
		currentLayer:             conf.Layers[0],
		execPressReleaseBindings: make(map[uint16]config.ExecPressReleaseBinding),
	}
	return &b
}

func (b *Executor) SetNextHandler(_ handlers.EventHandler) {
}

func (b *Executor) SetLayerManager(_ handlers.LayerManager) {
}

func (b *Executor) HandleEvent(eventBinding handlers.EventBinding) {
	// todo: check if reversing the order has side effects
	if !eventBinding.Event.IsPress {
		b.KeyReleased(eventBinding.Event.Code)
	}
	if eventBinding.Binding != nil {
		b.ExecuteBinding(eventBinding.Binding, eventBinding.Event.Code)
	}
}

func (b *Executor) ExecuteBinding(binding config.Binding, causeCode uint16) {
	log.Debugf("Executing %T: %+v", binding, binding)

	switch t := binding.(type) {
	case config.MultiBinding:
		for _, binding := range t.Bindings {
			b.ExecuteBinding(binding, causeCode)
		}
	case config.SpeedBinding:
		b.virtualMouse.AddSpeedFactor(causeCode, t.Speed)
	case config.ScrollBinding:
		b.virtualMouse.ChangeScrollSpeed(causeCode, t.X, t.Y)
	case config.MoveBinding:
		b.virtualMouse.ChangeMoveSpeed(causeCode, t.X, t.Y)
	case config.ButtonBinding:
		b.virtualMouse.ButtonPress(causeCode, t.Button)
	case config.KeyBinding:
		// replace any wildcard with the key that was pressed
		keys := make([]uint16, len(t.KeyCombo))
		copy(keys, t.KeyCombo)
		for i, key := range keys {
			if key == config.WildcardKey {
				keys[i] = causeCode
			}
		}
		b.virtualKeyboard.PressKeys(causeCode, keys)
	case config.KeyPressBinding:
		b.virtualKeyboard.PressKeyManually(t.Key)
	case config.KeyReleaseBinding:
		b.virtualKeyboard.ReleaseKeyManually(t.Key)
	case config.LayerBinding:
		// deactivate any toggled layers
		if b.toggleLayerPrevious != nil {
			b.toggleLayerKeys = nil
			b.toggleLayerPrevious = nil
		}
		for _, layer := range b.config.Layers {
			if layer.Name == t.Layer {
				b.goToLayer(layer)
				break
			}
		}
	case config.ToggleLayerBinding:
		for _, layer := range b.config.Layers {
			if layer.Name == t.Layer {
				b.toggleLayerKeys = append(b.toggleLayerKeys, causeCode)
				b.toggleLayerPrevious = append(b.toggleLayerPrevious, b.currentLayer)
				b.goToLayer(layer)
				break
			}
		}
	case config.ReloadConfigBinding:
		select {
		case b.reloadConfigChannel <- struct{}{}:
		default:
		}
	case config.ExecBinding:
		log.Debugf("Executing: %s", t.Command)
		executeCommandWithKey(t.Command, causeCode)
	case config.ExecPressReleaseBinding:
		log.Debugf("Executing: %s", t.PressCommand)
		executeCommandWithKey(t.PressCommand, causeCode)
		b.execPressReleaseBindings[causeCode] = t
	}
}

func (b *Executor) CurrentLayer() *config.Layer {
	return b.currentLayer
}

func (b *Executor) BaseLayer() *config.Layer {
	return b.config.Layers[0]
}

func (b *Executor) GetLayer(name string) (*config.Layer, bool) {
	for _, layer := range b.config.Layers {
		if layer.Name == name {
			return layer, true
		}
	}
	return nil, false
}

func (b *Executor) KeyReleased(code uint16) {
	// go back to the previous layer when toggleLayerKey is released
	for i, key := range b.toggleLayerKeys {
		if key == code {
			b.goToLayer(b.toggleLayerPrevious[i])
			// all layers that have been toggled after the current one are removed as well
			b.toggleLayerKeys = b.toggleLayerKeys[:i]
			b.toggleLayerPrevious = b.toggleLayerPrevious[:i]
			break
		}
	}

	// execute ExecPressReleaseBindings
	if binding, ok := b.execPressReleaseBindings[code]; ok {
		executeCommandWithKey(binding.ReleaseCommand, code)
		delete(b.execPressReleaseBindings, code)
	}

	// inform the keyboard and mouse about key releases
	b.virtualKeyboard.OriginalKeyUp(code)
	b.virtualMouse.OriginalKeyUp(code)
}

// goToLayer switches to the given layer and executes the appropriate exit and enter commands if set.
func (b *Executor) goToLayer(layer *config.Layer) {
	if b.currentLayer.ExitCommand != nil {
		executeCommand(*b.currentLayer.ExitCommand)
	}
	log.Debugf("Switching to layer %v", layer.Name)
	b.currentLayer = layer
	if layer.EnterCommand != nil {
		executeCommand(*layer.EnterCommand)
	}
}

// executeCommandWithKey executes the given command with the given key as environment variable.
func executeCommandWithKey(command string, causeCode uint16) {
	alias, exists := config.GetKeyAlias(causeCode)
	if !exists {
		alias = "unknown"
	}
	executeCommand(command, fmt.Sprintf("key=%s", alias), fmt.Sprintf("key_code=%d", causeCode))
}

// executeCommand executes the given command with the given environment variables.
func executeCommand(command string, envs ...string) {
	log.Debugf("Executing command: %s", command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append(os.Environ(), envs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Warnf("Execution of command '%s' failed: %v, stderr: %s", command, err, stderr.String())
	}
}
