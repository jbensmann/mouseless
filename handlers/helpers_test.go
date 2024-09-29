package handlers

import (
	"fmt"
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/keyboard"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type EventHandlerMock struct {
	layers       []*config.Layer
	currentLayer string

	toggleLayerKeys     []uint16
	toggleLayerPrevious []string

	eventBindings []EventBinding
}

func NewEventHandlerMock(conf *config.Config) *EventHandlerMock {
	return &EventHandlerMock{
		layers:       conf.Layers,
		currentLayer: "1",
	}
}

func (b *EventHandlerMock) HandleEvent(eventBinding EventBinding) {
	b.eventBindings = append(b.eventBindings, eventBinding)
	event := eventBinding.Event
	binding := eventBinding.Binding

	if event.IsPress {
		// check for toggle-layer binding
		if binding, ok := binding.(config.ToggleLayerBinding); ok {
			b.currentLayer = binding.Layer
			b.toggleLayerKeys = append(b.toggleLayerKeys, event.Code)
			b.toggleLayerPrevious = append(b.toggleLayerPrevious, b.currentLayer)
		}
	} else {
		// go back to the previous layer when toggleLayerKey is released
		for i, key := range b.toggleLayerKeys {
			if key == event.Code {
				b.currentLayer = b.toggleLayerPrevious[i]
				// all layers that have been toggled after the current one are removed as well
				b.toggleLayerKeys = b.toggleLayerKeys[:i]
				b.toggleLayerPrevious = b.toggleLayerPrevious[:i]
				return
			}
		}
	}
}

func (b *EventHandlerMock) BaseLayer() *config.Layer {
	return b.layers[0]
}

func (b *EventHandlerMock) CurrentLayer() *config.Layer {
	for _, layer := range b.layers {
		if layer.Name == b.currentLayer {
			return layer
		}
	}
	panic(fmt.Sprintf("non existing layer: %s", b.currentLayer))
}

func (b *EventHandlerMock) SetNextHandler(_ EventHandler) {
}

func (b *EventHandlerMock) SetLayerManager(_ LayerManager) {
}

func testHandler(t *testing.T, handlerGenerator func() EventHandler, configStr string, tests [][]string) {
	conf, err := config.ParseConfig([]byte(configStr))
	if err != nil {
		t.Fatal(fmt.Sprintf("Error parsing config: %v", err))
	}
	for _, test := range tests {
		handler := handlerGenerator()
		testCase(t, handler, conf, test[0], test[1])
	}
}

func testCase(t *testing.T, handler EventHandler, conf *config.Config, events string, expectedEventBindings string) {
	handlerMock := NewEventHandlerMock(conf)

	handler.SetLayerManager(handlerMock)
	handler.SetNextHandler(handlerMock)

	// feed events in
	feedEventsIn(handler, events)

	// parse the expected event bindings
	var expEventBindings []EventBinding
	for _, exp := range strings.Split(expectedEventBindings, " ") {
		expEventBindings = append(expEventBindings, parseEventBinding(exp))
	}

	// check if we received the expected number of events
	if len(expEventBindings) != len(handlerMock.eventBindings) {
		t.Errorf(
			"expected %d event bindings but got %d for test case (%s, %s)",
			len(expEventBindings),
			len(handlerMock.eventBindings),
			events,
			expectedEventBindings,
		)
		return
	}

	// check if events and bindings are the same (except the time)
	for i, expEventBinding := range expEventBindings {
		actEventBinding := handlerMock.eventBindings[i]
		if actEventBinding.Event.Code != expEventBinding.Event.Code || actEventBinding.Event.IsPress != expEventBinding.Event.IsPress ||
			!reflect.DeepEqual(actEventBinding.Binding, expEventBinding.Binding) {
			t.Errorf(
				"expected (%s,%+v) but got (%s,%+v) at index %d for test case (%s, %s)",
				convertEventToString(expEventBinding.Event),
				expEventBinding.Binding,
				convertEventToString(actEventBinding.Event),
				actEventBinding.Binding,
				i,
				events,
				expectedEventBindings,
			)
		}
	}
}

func convertEventToString(event keyboard.Event) string {
	alias, _ := config.GetKeyAlias(event.Code)
	prefix := "R"
	if event.IsPress {
		prefix = "P"
	}
	return fmt.Sprintf("%s%s", prefix, alias)
}

func feedEventsIn(handler EventHandler, events string) {
	for _, s := range strings.Split(events, " ") {
		if s[0] == 'P' || s[0] == 'R' {
			eventBinding := parseEventBinding(s)
			handler.HandleEvent(eventBinding)
		} else {
			ms, err := strconv.ParseUint(s, 10, 16)
			if err != nil {
				panic(fmt.Sprintf("failed to parse milliseconds: %s", s))
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	}
}

func parseEventBinding(eventBinding string) EventBinding {
	split := strings.Split(eventBinding, ":")
	code, _ := config.GetKeyCode(split[0][1:])
	event := keyboard.Event{
		Code:    code,
		IsPress: split[0][0] == 'P',
		Time:    time.Now(),
	}
	var binding config.Binding
	if len(split) > 1 {
		b := split[1]
		if b[0] == 'K' {
			code, _ = config.GetKeyCode(b[1:])
			binding = config.KeyBinding{KeyCombo: []uint16{code}}
		} else if b[0] == 'L' {
			binding = config.ToggleLayerBinding{Layer: b[1:]}
		} else if b[0] == 'N' {
			binding = config.NopBinding{}
		} else {
			panic(fmt.Sprintf("unexpected binding type %v", b[0]))
		}
	}
	return EventBinding{Event: event, Binding: binding}
}
