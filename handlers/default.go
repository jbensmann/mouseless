package handlers

import (
	evdev "github.com/gvalkov/golang-evdev"
	"github.com/jbensmann/mouseless/config"
	log "github.com/sirupsen/logrus"
)

type DefaultHandler struct {
	BaseHandler
}

func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{}
}

func (d *DefaultHandler) HandleEvent(eventBinding EventBinding) {
	log.Debugf("DefaultHandler: handling Event: %+v", eventBinding)
	event := eventBinding.Event

	// resolve the Binding if it is a press and not bound yet
	if event.IsPress && eventBinding.Binding == nil {
		currentLayer := d.layerManager.CurrentLayer()
		binding, _ := currentLayer.Bindings[event.Code]

		// switch to first layer on escape, if not mapped to something else
		baseLayer := d.layerManager.BaseLayer()
		if binding == nil && event.Code == evdev.KEY_ESC && currentLayer != baseLayer {
			binding = config.LayerBinding{Layer: baseLayer.Name}
		}

		// use the wildcard Binding if no Binding is defined for the key
		if binding == nil && currentLayer.WildcardBinding != nil {
			binding = currentLayer.WildcardBinding
		}

		// if there is no wildcard either and pass through is enabled, insert a KeyBinding
		if binding == nil && currentLayer.PassThrough {
			binding = config.KeyBinding{KeyCombo: []uint16{event.Code}}
		}

		eventBinding.Binding = binding
	}

	d.next.HandleEvent(eventBinding)
}
