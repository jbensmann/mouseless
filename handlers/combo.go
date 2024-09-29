package handlers

import (
	"sync"
	"time"

	"github.com/jbensmann/mouseless/config"
	log "github.com/sirupsen/logrus"
)

type ComboState int

const (
	ComboStateIdle ComboState = iota
	ComboStateWait
	ComboStateNoCombo
	ComboStateCombo
)

type ComboHandler struct {
	BaseHandler

	mu sync.Mutex

	comboTime int64

	// store all incoming events in a queue first, as in the TapHoldHandler (it is never greater than 2 in this case)
	// use pointers so that we can edit the binding
	eventInQueue    []*EventBinding
	eventInPosition int

	state         ComboState
	comboTimer    *time.Timer
	comboBindings map[uint16]config.Binding
}

func NewComboHandler(comboTime int64) *ComboHandler {
	handler := ComboHandler{
		comboTime:       comboTime,
		eventInPosition: 0,
		state:           ComboStateIdle,
	}
	return &handler
}

func (c *ComboHandler) HandleEvent(event EventBinding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventInQueue = append(c.eventInQueue, &event)
	c.handleEvents()
}

func (c *ComboHandler) handleEvents() {
	for c.eventInPosition <= len(c.eventInQueue)-1 {
		c.handleNextEvent()
	}
}

func (c *ComboHandler) comboTimeout() {
	timer := c.comboTimer
	c.mu.Lock()
	defer c.mu.Unlock()

	// check if the timer has been stopped while waiting for the lock
	if timer == nil || timer != c.comboTimer {
		return
	}
	log.Debugf("ComboHandler: timed out")
	c.state = ComboStateNoCombo
	c.comboResolved()
	c.handleEvents()
}

func (c *ComboHandler) handleNextEvent() {
	eventBinding := c.eventInQueue[c.eventInPosition]
	event := eventBinding.Event

	log.Debugf("ComboHandler: handling Event: %+v", eventBinding)

	comboBindings, isComboBinding := c.checkForComboBinding(*eventBinding)

	if event.IsPress {
		if isComboBinding {
			if c.state != ComboStateWait {
				log.Debugf("ComboHandler: waiting")
				c.state = ComboStateWait
				c.comboBindings = comboBindings

				// set timeout to the defined timeout minus the already passed duration since the key press
				timeout := time.Duration(c.comboTime)*time.Millisecond - time.Now().Sub(event.Time)
				if timeout < 0 {
					timeout = 0
				}
				c.comboTimer = time.AfterFunc(timeout, c.comboTimeout)
			}
		}
	} else {
		if c.state == ComboStateWait {
			// don't execute combo if the first key is released
			if c.eventInQueue[0].Event.Code == event.Code {
				c.state = ComboStateNoCombo
			}
		}
	}

	// if another key is pressed, check if it is a combo
	if c.state == ComboStateWait && event.Code != c.eventInQueue[0].Event.Code {
		if event.IsPress {
			if binding, ok := c.comboBindings[event.Code]; ok {
				c.eventInQueue[0].Binding = binding
				// the second key is consumed now
				eventBinding.Binding = config.NopBinding{}

				c.state = ComboStateCombo
			} else {
				c.state = ComboStateNoCombo
			}
		} else {
			c.state = ComboStateNoCombo
		}
	}

	if c.state == ComboStateNoCombo || c.state == ComboStateCombo {
		c.comboResolved()
	} else if c.state == ComboStateIdle {
		// forward Event unchanged to the next handler
		c.EventHandled(*eventBinding)

		// remove the eventBinding from eventInQueue
		c.eventInQueue = append(c.eventInQueue[:c.eventInPosition], c.eventInQueue[c.eventInPosition+1:]...)
	} else {
		// state ComboStateWait
		// move to the next Event
		c.eventInPosition += 1
	}
}

// / comboResolved must be called after the state changed to ComboStateCombo or ComboStateNoCombo.
func (c *ComboHandler) comboResolved() {
	// stop the comboTimer in case it has not fired yet
	if c.comboTimer != nil {
		c.comboTimer.Stop()
		c.comboTimer = nil
	}

	// the first key in the queue is the one that triggered the combo
	c.EventHandled(*c.eventInQueue[0])
	c.eventInQueue = c.eventInQueue[1:]

	if c.state == ComboStateNoCombo {
		log.Debugf("ComboHandler: no combo")
	} else {
		log.Debugf("ComboHandler: combo triggered")
	}

	c.state = ComboStateIdle

	// start again from the beginning of the queue
	c.eventInPosition = 0
}

// checkForComboBinding checks if the given eventBinding is mapped to a combo in the current layer, and has
// no other Binding attached to it.
// If the check is positive, it returns a map with all combo bindings, where the key is the other event code of the combo.
// Otherwise, it returns nil.
func (c *ComboHandler) checkForComboBinding(eventBinding EventBinding) (map[uint16]config.Binding, bool) {
	if eventBinding.Binding != nil {
		return nil, false
	}
	currentLayer := c.layerManager.CurrentLayer()
	if comboBindings, ok := currentLayer.ComboBindings[eventBinding.Event.Code]; ok {
		return comboBindings, true
	}
	return nil, false
}

func (c *ComboHandler) EventHandled(eventBinding EventBinding) {
	c.next.HandleEvent(eventBinding)
}
