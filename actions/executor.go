package actions

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/handlers"
	"github.com/jbensmann/mouseless/keyboard"
	"github.com/jbensmann/mouseless/virtual"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

type ExecutedBinding struct {
	cause   *keyboard.Event
	binding config.Binding
}

type BindingExecutor struct {
	config              *config.Config
	virtualKeyboard     *virtual.VirtualKeyboard
	virtualMouse        *virtual.Mouse
	reloadConfigChannel chan<- struct{}

	currentLayer *config.Layer
	// remember all keys that toggled a layer, and from which layer they came from
	toggleLayerKeys     []uint16
	toggleLayerPrevious []*config.Layer
}

func NewBindingExecutor(config *config.Config, virtualKeyboard *virtual.VirtualKeyboard, virtualMouse *virtual.Mouse,
	reloadConfigChannel chan struct{}) *BindingExecutor {
	b := BindingExecutor{
		config:              config,
		virtualKeyboard:     virtualKeyboard,
		virtualMouse:        virtualMouse,
		reloadConfigChannel: reloadConfigChannel,
		currentLayer:        config.Layers[0],
	}
	return &b
}

func (b *BindingExecutor) SetNextHandler(_ handlers.EventHandler) {
}

func (b *BindingExecutor) SetLayerManager(_ handlers.LayerManager) {
}

func (b *BindingExecutor) HandleEvent(eventBinding handlers.EventBinding) {
	if eventBinding.Binding != nil {
		b.ExecuteBinding(eventBinding.Binding, eventBinding.Event.Code)
	}
	if !eventBinding.Event.IsPress {
		b.KeyReleased(eventBinding.Event.Code)
	}
}

func (b *BindingExecutor) ExecuteBinding(binding config.Binding, causeCode uint16) {
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
		cmd := exec.Command("sh", "-c", t.Command)
		// pass the pressed key as environment variable
		alias, exists := config.GetKeyAlias(causeCode)
		if !exists {
			alias = "unknown"
		}
		cmd.Env = append(
			os.Environ(),
			fmt.Sprintf("key=%s", alias),
			fmt.Sprintf("key_code=%d", causeCode),
		)
		err := cmd.Run()
		if err != nil {
			log.Warnf("Execution of command failed: %v", err)
		}
	}
}

func (b *BindingExecutor) CurrentLayer() *config.Layer {
	return b.currentLayer
}

func (b *BindingExecutor) BaseLayer() *config.Layer {
	return b.config.Layers[0]
}

func (b *BindingExecutor) KeyReleased(code uint16) {
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

	// inform the keyboard and mouse about key releases
	b.virtualKeyboard.OriginalKeyUp(code)
	b.virtualMouse.OriginalKeyUp(code)
}

// goToLayer switches to the given layer and executes the appropriate exit and enter commands if set.
func (b *BindingExecutor) goToLayer(layer *config.Layer) {
	executeCommandIfNotEmpty(b.currentLayer.ExitCommand)
	log.Debugf("Switching to layer %v", layer.Name)
	b.currentLayer = layer
	executeCommandIfNotEmpty(layer.EnterCommand)
}

func executeCommandIfNotEmpty(command *string) {
	if command != nil && *command != "" {
		log.Debugf("Executing command: %s", *command)
		cmd := exec.Command("sh", "-c", *command)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				log.Warnf("Execution of command '%s' failed: %v, stderr: %s", *command, err, stderr.String())
			} else {
				log.Warnf("Execution of command '%s' failed: %v", *command, err)
			}
		}
	}
}
