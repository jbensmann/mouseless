package handlers

import (
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/keyboard"
)

type EventBinding struct {
	Event   keyboard.Event
	Binding config.Binding
}

type LayerManager interface {
	CurrentLayer() *config.Layer
	BaseLayer() *config.Layer
}

type EventHandler interface {
	HandleEvent(event EventBinding)
	SetNextHandler(handler EventHandler)
	SetLayerManager(manager LayerManager)
}

type BaseHandler struct {
	next         EventHandler
	layerManager LayerManager
}

func (b *BaseHandler) SetNextHandler(handler EventHandler) {
	b.next = handler
}

func (b *BaseHandler) SetLayerManager(manager LayerManager) {
	b.layerManager = manager
}
