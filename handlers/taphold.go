package handlers

import (
	"sync"
	"time"

	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/keyboard"

	log "github.com/sirupsen/logrus"
)

type TapHoldState int

const (
	TapHoldStateIdle TapHoldState = iota
	TapHoldStateWait
	TapHoldStateTap
	TapHoldStateHold
)

type TapHoldHandler struct {
	BaseHandler

	quickTapTime int64

	eventInQueue    []EventBinding
	eventInPosition int
	eventHandleLock sync.Mutex

	isPressed     map[uint16]bool
	isPressedLock sync.RWMutex
	lastPressed   map[uint16]time.Time

	state                  TapHoldState
	tapHoldBinding         *config.TapHoldBinding
	tapHoldTimer           *time.Timer
	holdBackStartIsPressed map[uint16]bool
}

func NewTapHoldHandler(quickTapTime int64) *TapHoldHandler {
	handler := TapHoldHandler{
		quickTapTime:           quickTapTime,
		eventInPosition:        0,
		state:                  TapHoldStateIdle,
		isPressed:              make(map[uint16]bool),
		lastPressed:            make(map[uint16]time.Time),
		holdBackStartIsPressed: make(map[uint16]bool),
	}
	return &handler
}

func (t *TapHoldHandler) HandleEvent(event EventBinding) {
	t.eventHandleLock.Lock()
	defer t.eventHandleLock.Unlock()
	t.eventInQueue = append(t.eventInQueue, event)
	t.handleEvents()
}

func (t *TapHoldHandler) handleEvents() {
	for t.eventInPosition <= len(t.eventInQueue)-1 {
		t.handleNextEvent()
	}
}

func (t *TapHoldHandler) tapHoldTimeout() {
	t.eventHandleLock.Lock()
	defer t.eventHandleLock.Unlock()
	// usually we are in the wait state, but there is a chance that it has already been resolved, but the timer has not yet been stopped
	// todo: check if it is possible that we are already in the next wait state
	if t.state == TapHoldStateWait {
		log.Debugf("TapHoldHandler: tapHold timed out")
		t.state = TapHoldStateHold

		t.handleEvents()
	}
}

func (t *TapHoldHandler) handleNextEvent() {
	eventBinding := t.eventInQueue[t.eventInPosition]
	event := eventBinding.Event

	log.Debugf("TapHoldHandler: handling Event: %+v", eventBinding)

	tapHoldBinding, isTapHoldBinding := t.checkForTapHoldBinding(eventBinding)

	if event.IsPress {
		// tapHold key pressed?
		if isTapHoldBinding {
			if t.state != TapHoldStateWait {
				log.Debugf("TapHoldHandler: activating holdBack")
				t.state = TapHoldStateWait
				t.tapHoldBinding = &tapHoldBinding

				// remember all pressed keys
				t.holdBackStartIsPressed = make(map[uint16]bool)
				t.isPressedLock.RLock()
				for k, v := range t.isPressed {
					t.holdBackStartIsPressed[k] = v
				}
				t.isPressedLock.RUnlock()

				// set timeout to the defined timeout minus the already passed duration since the key press
				if tapHoldBinding.TimeoutMs > 0 {
					timeout := time.Duration(tapHoldBinding.TimeoutMs)*time.Millisecond - time.Now().Sub(event.Time)
					if timeout < 0 {
						timeout = 0
					}
					t.tapHoldTimer = time.AfterFunc(timeout, t.tapHoldTimeout)
				}

				// if the key has been pressed recently within quickTapTime, activate the tap Binding
				lastPressed, isPressed := t.lastPressed[event.Code]
				recentlyPressed := isPressed && event.Time.Before(lastPressed.Add(time.Duration(t.quickTapTime)*time.Millisecond))
				if recentlyPressed {
					t.state = TapHoldStateTap
				}
			}
		}
	} else {
		if t.state == TapHoldStateWait {
			// execute tap Binding if tapHold key released
			if t.tapHoldBinding != nil && t.eventInQueue[0].Event.Code == event.Code {
				t.state = TapHoldStateTap
			}
		}
	}

	// if tapOnNext and another key is pressed, activate tap hold
	if t.state == TapHoldStateWait && event.Code != t.eventInQueue[0].Event.Code {
		if event.IsPress {
			if t.tapHoldBinding.TapOnNext {
				t.state = TapHoldStateHold
			}
		} else {
			if t.tapHoldBinding.TapOnNextRelease {
				if _, ok := t.holdBackStartIsPressed[event.Code]; !ok {
					t.state = TapHoldStateHold
				}
			}
		}
	}

	if t.state == TapHoldStateHold || t.state == TapHoldStateTap {
		// a tap or hold Binding was activated

		// stop the tapHoldTimer in case it has not fired yet
		if t.tapHoldTimer != nil {
			t.tapHoldTimer.Stop()
			t.tapHoldTimer = nil
		}

		// the first key in holdBackEvents is the one that triggered the tap-hold
		tapHoldEventBinding := t.eventInQueue[0]
		t.eventInQueue = t.eventInQueue[1:]

		if t.state == TapHoldStateHold {
			log.Debugf("TapHoldHandler: activated hold Binding")
			tapHoldEventBinding.Binding = t.tapHoldBinding.HoldBinding
		} else {
			log.Debugf("TapHoldHandler: activated tap Binding")
			tapHoldEventBinding.Binding = t.tapHoldBinding.TapBinding
		}
		t.EventHandled(tapHoldEventBinding)

		t.state = TapHoldStateIdle
		t.tapHoldBinding = nil

		// process from the beginning of the queue
		t.eventInPosition = 0
	} else if t.state == TapHoldStateIdle {
		// forward Event unchanged to the next handler
		t.EventHandled(eventBinding)

		// remove the eventBinding from eventInQueue
		t.eventInQueue = append(t.eventInQueue[:t.eventInPosition], t.eventInQueue[t.eventInPosition+1:]...)
	} else {
		// state TapHoldStateWait
		// move to the next Event
		t.eventInPosition += 1
	}
}

// checkForTapHoldBinding checks if the given eventBinding is mapped to a TapHoldBinding in the current layer, and has
// no other Binding attached to it.
// If the check is positive, it returns the TapHoldBinding.
// Otherwise, it returns nil.
func (t *TapHoldHandler) checkForTapHoldBinding(eventBinding EventBinding) (config.TapHoldBinding, bool) {
	var mappedBinding config.Binding
	if eventBinding.Binding != nil {
		mappedBinding = eventBinding.Binding
	} else {
		currentLayer := t.layerManager.CurrentLayer()
		mappedBinding, _ = currentLayer.Bindings[eventBinding.Event.Code]
	}
	log.Debugf("mapped binding: %+v", mappedBinding)
	if tapHoldBinding, ok := mappedBinding.(config.TapHoldBinding); ok {
		return tapHoldBinding, true
	} else {
		return config.TapHoldBinding{}, false
	}
}

func (t *TapHoldHandler) EventHandled(eventBinding EventBinding) {
	t.setKeyPressed(eventBinding.Event)
	t.next.HandleEvent(eventBinding)
}

func (t *TapHoldHandler) setKeyPressed(event keyboard.Event) {
	t.isPressedLock.Lock()
	defer t.isPressedLock.Unlock()
	if event.IsPress {
		t.isPressed[event.Code] = true
		t.lastPressed[event.Code] = event.Time
	} else {
		delete(t.isPressed, event.Code)
	}
	if _, ok := t.holdBackStartIsPressed[event.Code]; ok {
		if !event.IsPress {
			delete(t.holdBackStartIsPressed, event.Code)
		}
	}
}

func (t *TapHoldHandler) IsKeyPressed(Code uint16) bool {
	t.isPressedLock.RLock()
	defer t.isPressedLock.RUnlock()
	pr, ok := t.isPressed[Code]
	return ok && pr
}
