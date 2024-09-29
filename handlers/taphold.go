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

	mu sync.Mutex

	quickTapTime int64

	eventInQueue    []*EventBinding
	eventInPosition int

	isPressed   map[uint16]struct{}
	lastPressed map[uint16]time.Time

	state                  TapHoldState
	tapHoldBinding         *config.TapHoldBinding
	tapHoldTimer           *time.Timer
	holdBackStartIsPressed map[uint16]struct{}
}

func NewTapHoldHandler(quickTapTime int64) *TapHoldHandler {
	handler := TapHoldHandler{
		quickTapTime:           quickTapTime,
		eventInPosition:        0,
		state:                  TapHoldStateIdle,
		isPressed:              make(map[uint16]struct{}),
		lastPressed:            make(map[uint16]time.Time),
		holdBackStartIsPressed: make(map[uint16]struct{}),
	}
	return &handler
}

func (t *TapHoldHandler) HandleEvent(event EventBinding) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventInQueue = append(t.eventInQueue, &event)
	t.handleEvents()
}

func (t *TapHoldHandler) handleEvents() {
	for t.eventInPosition <= len(t.eventInQueue)-1 {
		t.handleNextEvent()
	}
}

func (t *TapHoldHandler) tapHoldTimeout() {
	timer := t.tapHoldTimer
	t.mu.Lock()
	defer t.mu.Unlock()

	// check if the timer has been stopped while waiting for the lock
	if timer == nil || timer != t.tapHoldTimer {
		return
	}
	log.Debugf("TapHoldHandler: tapHold timed out")
	t.state = TapHoldStateHold
	t.resolveTapHold()
	t.handleEvents()
}

func (t *TapHoldHandler) handleNextEvent() {
	eventBinding := t.eventInQueue[t.eventInPosition]
	event := eventBinding.Event

	log.Debugf("TapHoldHandler: handling Event: %+v", eventBinding)

	tapHoldBinding, isTapHoldBinding := t.checkForTapHoldBinding(*eventBinding)

	if event.IsPress {
		// tapHold key pressed?
		if isTapHoldBinding {
			if t.state != TapHoldStateWait {
				log.Debugf("TapHoldHandler: activating holdBack")
				t.state = TapHoldStateWait
				t.tapHoldBinding = &tapHoldBinding

				// remember all pressed keys
				t.holdBackStartIsPressed = make(map[uint16]struct{})
				for k, v := range t.isPressed {
					t.holdBackStartIsPressed[k] = v
				}

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
					log.Debugf("TapHoldHandler: quick tap detected")
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

	if t.state == TapHoldStateWait && event.Code != t.eventInQueue[0].Event.Code {
		if event.IsPress {
			// if TapOnNext and another key is pressed, activate tap hold
			if t.tapHoldBinding.TapOnNext {
				t.state = TapHoldStateHold
			}
		} else {
			// if TapOnNextRelease and another key is released that wasn't pressed before the tap key, activate tap hold
			if t.tapHoldBinding.TapOnNextRelease {
				if _, ok := t.holdBackStartIsPressed[event.Code]; !ok {
					t.state = TapHoldStateHold
				}
			}
		}
	}

	if t.state == TapHoldStateHold || t.state == TapHoldStateTap {
		t.resolveTapHold()
	} else if t.state == TapHoldStateIdle {
		// forward Event unchanged to the next handler
		t.eventHandled(0)
	} else {
		// state TapHoldStateWait
		_, wasPressed := t.holdBackStartIsPressed[event.Code]
		if !event.IsPress && wasPressed {
			// forward a key release where the press was before the tap hold started
			// todo: make this configurable?
			log.Debugf("TapHoldHandler: forwarding key release %v which was pressed before the tap hold started", event.Code)
			t.eventHandled(t.eventInPosition)
		} else {
			// move to the next Event
			t.eventInPosition += 1
		}
	}
}

// resolveTapHold must be called when a TapHoldBinding has been resolved.
func (t *TapHoldHandler) resolveTapHold() {
	// should only be called in state TapHoldStateTap or TapHoldStateHold
	if t.state != TapHoldStateTap && t.state != TapHoldStateHold {
		log.Debugf("TapHoldHandler: resolveTapHold called in state %v", t.state)
		return
	}

	// stop the tapHoldTimer in case it has not fired yet
	if t.tapHoldTimer != nil {
		t.tapHoldTimer.Stop()
		t.tapHoldTimer = nil
	}

	// the first key in holdBackEvents is the one that triggered the tap-hold
	tapHoldEventBinding := t.eventInQueue[0]

	if t.state == TapHoldStateHold {
		log.Debugf("TapHoldHandler: activated hold Binding")
		tapHoldEventBinding.Binding = t.tapHoldBinding.HoldBinding
	} else {
		log.Debugf("TapHoldHandler: activated tap Binding")
		tapHoldEventBinding.Binding = t.tapHoldBinding.TapBinding
	}
	t.eventHandled(0)

	t.state = TapHoldStateIdle
	t.tapHoldBinding = nil

	// process from the beginning of the queue
	t.eventInPosition = 0
}

// checkForTapHoldBinding checks if the given eventBinding is mapped to a TapHoldBinding in the current layer or has
// already a TapHoldBinding attached to it.
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
	if tapHoldBinding, ok := mappedBinding.(config.TapHoldBinding); ok {
		return tapHoldBinding, true
	} else {
		return config.TapHoldBinding{}, false
	}
}

// eventHandled handles the event at position and removes it from the queue.
func (t *TapHoldHandler) eventHandled(position int) {
	if position >= len(t.eventInQueue) {
		log.Errorf("Unexpected interal error in eventHandled: given position %d, but len(eventInQueue) is %d",
			position, len(t.eventInQueue))
		return
	}
	eventBinding := t.eventInQueue[position]
	t.setKeyPressed(eventBinding.Event)
	t.next.HandleEvent(*eventBinding)

	// remove the eventBinding from eventInQueue
	t.eventInQueue = append(t.eventInQueue[:position], t.eventInQueue[position+1:]...)
}

// setKeyPressed updates the internal state of which keys are pressed.
func (t *TapHoldHandler) setKeyPressed(event keyboard.Event) {
	if event.IsPress {
		t.isPressed[event.Code] = struct{}{}
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
