package handlers

import (
	"github.com/jbensmann/mouseless/config"

	log "github.com/sirupsen/logrus"
)

type ModLayerState int

const (
	ModLayerStateIdle ModLayerState = iota
	ModLayerStateModActive
	ModLayerStateLayerActive
)

type ModLayerHandler struct {
	BaseHandler
	state            ModLayerState
	modLayerBinding  *config.ModLayerBinding
	triggerKey       uint16
	layer            *config.Layer
	originalLayer    *config.Layer
	pressedLayerKeys map[uint16]struct{}
}

func NewModLayerHandler() *ModLayerHandler {
	handler := ModLayerHandler{
		state: ModLayerStateIdle,
	}
	return &handler
}

func (t *ModLayerHandler) HandleEvent(eventBinding EventBinding) {
	log.Debugf("ModLayerHandler: handling Event: %+v", eventBinding)
	event := eventBinding.Event

	modLayerBinding, isModLayerBinding := t.checkForModLayerBinding(eventBinding)

	if event.IsPress {
		if isModLayerBinding {
			log.Debugf("ModLayerHandler: pressing modifier")
			eventBinding.Binding = config.KeyBinding{KeyCombo: []uint16{modLayerBinding.ModKey}}

			// we don't allow two active mod bindings at the same time
			if t.state == ModLayerStateIdle {
				layer, ok := t.layerManager.GetLayer(modLayerBinding.Layer)
				if !ok {
					log.Warnf("ModLayerHandler: layer does not exist: %s", modLayerBinding.Layer)
				} else {
					t.state = ModLayerStateModActive
					t.layer = layer
					t.modLayerBinding = &modLayerBinding
					t.triggerKey = event.Code
					t.pressedLayerKeys = make(map[uint16]struct{})
					t.originalLayer = t.layerManager.CurrentLayer()
				}
			}
		} else {
			// if a mod binding is active and the pressed key is mapped in the corresponding layer,
			// insert the binding, unless the layer has changed in the meantime
			if t.state != ModLayerStateIdle && t.layerManager.CurrentLayer() == t.originalLayer {
				binding, hasBinding := t.layer.Bindings[event.Code]
				if hasBinding {
					t.pressedLayerKeys[event.Code] = struct{}{}

					if t.state == ModLayerStateModActive {
						t.state = ModLayerStateLayerActive
						eventBinding.Binding = config.MultiBinding{
							Bindings: []config.Binding{
								config.KeyReleaseBinding{Key: t.modLayerBinding.ModKey},
								binding,
							},
						}
					} else {
						// ModLayerStateLayerActive
						eventBinding.Binding = binding
					}
				}
			}

		}
	} else {
		// key release
		if t.state != ModLayerStateIdle {
			// if no more mapped keys are pressed, press the mod key again
			if _, ok := t.pressedLayerKeys[event.Code]; ok {
				delete(t.pressedLayerKeys, event.Code)
				if t.state == ModLayerStateLayerActive && len(t.pressedLayerKeys) == 0 {
					t.state = ModLayerStateModActive
					eventBinding.Binding = config.KeyPressBinding{Key: t.modLayerBinding.ModKey}
				}
			}

			// done if the trigger key is released
			if t.modLayerBinding != nil && t.triggerKey == event.Code {
				log.Debugf("ModLayerHandler: done due to release of modifier")
				t.modLayerBinding = nil
				t.state = ModLayerStateIdle
				t.modLayerBinding = nil
				t.pressedLayerKeys = make(map[uint16]struct{})
				t.layer = nil
			}
		}
	}

	t.next.HandleEvent(eventBinding)
}

// checkForModLayerBinding checks if the given eventBinding is mapped to a ModLayerBinding in the current layer or has
// already a ModLayerBinding attached to it.
func (t *ModLayerHandler) checkForModLayerBinding(eventBinding EventBinding) (config.ModLayerBinding, bool) {
	var mappedBinding config.Binding
	if eventBinding.Binding != nil {
		mappedBinding = eventBinding.Binding
	} else {
		currentLayer := t.layerManager.CurrentLayer()
		mappedBinding, _ = currentLayer.Bindings[eventBinding.Event.Code]
	}
	modLayerBinding, ok := mappedBinding.(config.ModLayerBinding)
	return modLayerBinding, ok
}
