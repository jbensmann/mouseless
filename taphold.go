package main

import (
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type TapHoldHandler struct {
	inChannel     chan KeyboardEvent
	outChannel    chan KeyboardEvent
	isPressed     map[uint16]bool
	isPressedLock sync.RWMutex

	isHoldBack             bool
	holdBackEvents         []*KeyboardEvent // pointers so that events can be manipulated
	holdBackTimeout        *time.Timer
	tapHoldEvent           *KeyboardEvent
	tapHoldBinding         *TapHoldBinding
	holdBackStartIsPressed map[uint16]bool
}

func NewTapHoldHandler() *TapHoldHandler {
	handler := TapHoldHandler{
		inChannel:              make(chan KeyboardEvent, 1000),
		outChannel:             make(chan KeyboardEvent, 1000),
		holdBackTimeout:        time.NewTimer(math.MaxInt64),
		isHoldBack:             false,
		isPressed:              make(map[uint16]bool),
		holdBackStartIsPressed: make(map[uint16]bool),
	}
	return &handler
}

func (t *TapHoldHandler) StartProcessing() {
	go func() {
		for {
			t.process()
		}
	}()
}

func (t *TapHoldHandler) process() {
	// wait for a key or tapHold timeout
	select {
	case e := <-t.inChannel:
		log.Debugf("Read from queue: %v", e)
		t.handleKey(e)
	case <-t.holdBackTimeout.C:
		log.Debugf("tapHold timed out")
		t.tapHoldEvent.holdKey = true
		t.isHoldBack = false
		t.holdBackTimeout = time.NewTimer(math.MaxInt64)
	}

	// when holdBack stopped, send all holdBack keys
	if !t.isHoldBack && len(t.holdBackEvents) > 0 {
		for _, e := range t.holdBackEvents {
			log.Debugf("Read from holdback queue: %v", e)
			t.outChannel <- *e
		}
		t.holdBackEvents = nil
	}
}

func (t *TapHoldHandler) handleKey(event KeyboardEvent) {
	previousIsHoldingBack := t.isHoldBack

	if event.isPress {
		// tapHold key pressed?
		if binding, ok := currentLayer.Bindings[event.code].(TapHoldBinding); ok {
			if !t.isHoldBack {
				log.Debugf("Activating holdBack")
				t.isHoldBack = true
				t.tapHoldEvent = &event
				t.tapHoldBinding = &binding

				// remember all pressed keys
				t.holdBackStartIsPressed = make(map[uint16]bool)
				t.isPressedLock.RLock()
				for k, v := range t.isPressed {
					t.holdBackStartIsPressed[k] = v
				}
				t.isPressedLock.RUnlock()

				// set timeout
				if binding.TimeoutMs > 0 {
					t.holdBackTimeout = time.NewTimer(time.Duration(binding.TimeoutMs) * time.Millisecond)
				}
			}
		}
	} else {
		// tapHold key released?
		if t.tapHoldBinding != nil && t.tapHoldEvent.code == event.code {
			if t.isHoldBack {
				// execute tap binding
				t.tapHoldEvent.holdKey = false
				t.isHoldBack = false
			}
		}
	}

	// if tapOnNext and another key is pressed, activate tap hold
	if t.isHoldBack && event.code != t.tapHoldEvent.code {
		if event.isPress {
			if t.tapHoldBinding.TapOnNext {
				t.isHoldBack = false
				t.tapHoldEvent.holdKey = true
			}
		} else {
			if t.tapHoldBinding.TapOnNextRelease {
				if _, ok := t.holdBackStartIsPressed[event.code]; !ok {
					t.isHoldBack = false
					t.tapHoldEvent.holdKey = true
				}
			}
		}
	}

	// send to out channel or to holdBackEvents
	holdBack := t.isHoldBack || previousIsHoldingBack
	// do not hold back if the key pressed before tap started to avoid unwanted key repetitions
	if _, ok := t.holdBackStartIsPressed[event.code]; ok {
		holdBack = false
	}
	if holdBack {
		log.Debugf("tapHold: putting event back to queue")
		t.holdBackEvents = append(t.holdBackEvents, &event)
	} else {
		t.outChannel <- event
	}

	// remember if key is pressed
	t.setKeyPressed(event.code, event.isPress)
}

func (t *TapHoldHandler) InChannel() chan<- KeyboardEvent {
	return t.inChannel
}

func (t *TapHoldHandler) OutChannel() <-chan KeyboardEvent {
	return t.outChannel
}

func (t *TapHoldHandler) IsKeyPressed(code uint16) bool {
	t.isPressedLock.RLock()
	defer t.isPressedLock.RUnlock()
	pr, ok := t.isPressed[code]
	return ok && pr
}

func (t *TapHoldHandler) setKeyPressed(code uint16, pressed bool) {
	t.isPressedLock.Lock()
	defer t.isPressedLock.Unlock()
	if pressed {
		t.isPressed[code] = true
	} else {
		delete(t.isPressed, code)
	}
}
