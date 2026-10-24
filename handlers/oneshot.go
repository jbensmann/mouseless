package handlers

import (
	"slices"
	"time"

	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/mouseless/keyboard"

	log "github.com/sirupsen/logrus"
)

type OneShotState int

const (
	// OneShotStateHeld means that the one-shot key is pressed for the first time.
	OneShotStateHeld OneShotState = iota
	// OneShotStateArmed means that the one-shot key has been tapped, so its binding is active until the next key press.
	OneShotStateArmed
	// OneShotStateArmedHeld means that the one-shot key is pressed again while being armed.
	OneShotStateArmedHeld
	// OneShotStateLocked means that the one-shot key has been tapped twice, so its binding is active until it is
	// tapped again.
	OneShotStateLocked
	// OneShotStateLockedHeld means that the one-shot key is pressed again while being locked.
	OneShotStateLockedHeld
)

type oneShotKey struct {
	code            uint16
	state           OneShotState
	otherKeyPressed bool
}

// OneShotHandler handles OneShotBindings: the binding is executed when the key is pressed, and when the key is tapped,
// it stays active until the next key press, or until it is tapped again in case of a double tap.
// It has to come after the DefaultHandler, since it relies on the bindings of key presses being resolved.
// The binding is kept active by holding back the release of the one-shot key, which is sent once the binding shall be
// deactivated.
type OneShotHandler struct {
	BaseHandler
	keys []*oneShotKey
}

func NewOneShotHandler() *OneShotHandler {
	return &OneShotHandler{}
}

func (o *OneShotHandler) HandleEvent(eventBinding EventBinding) {
	log.Debugf("OneShotHandler: handling Event: %+v", eventBinding)
	event := eventBinding.Event
	key := o.getKey(event.Code)

	if event.IsPress {
		oneShotBinding, isOneShot := eventBinding.Binding.(config.OneShotBinding)

		if key != nil {
			if isOneShot || isUnboundKey(eventBinding.Binding, event.Code) {
				// an active one-shot key is pressed again, its binding is still active
				switch key.state {
				case OneShotStateArmed:
					key.state = OneShotStateArmedHeld
				case OneShotStateLocked:
					key.state = OneShotStateLockedHeld
				}
				key.otherKeyPressed = false
				return
			}
			// the key is bound to another action now (e.g. the hold action of a tap-hold), so release the one-shot
			o.releaseKey(key)
		}

		// presses mapped to nop (e.g. the second key of a combo) are ignored
		if _, isNop := eventBinding.Binding.(config.NopBinding); isNop {
			o.next.HandleEvent(eventBinding)
			return
		}

		for _, k := range o.keys {
			k.otherKeyPressed = true
		}

		if isOneShot {
			log.Debugf("OneShotHandler: one-shot key pressed")
			o.keys = append(o.keys, &oneShotKey{code: event.Code, state: OneShotStateHeld})
			eventBinding.Binding = oneShotBinding.Binding
			o.next.HandleEvent(eventBinding)
			return
		}

		o.next.HandleEvent(eventBinding)
		// modifiers don't consume armed one-shot keys, so that they can be combined
		if !isModifierBinding(eventBinding.Binding) {
			o.releaseArmedKeys()
		}
		return
	}

	// key release
	if key != nil {
		switch key.state {
		case OneShotStateHeld:
			if !key.otherKeyPressed {
				log.Debugf("OneShotHandler: one-shot key tapped, arming it")
				key.state = OneShotStateArmed
				return
			}
		case OneShotStateArmedHeld:
			if !key.otherKeyPressed {
				log.Debugf("OneShotHandler: one-shot key tapped twice, locking it")
				key.state = OneShotStateLocked
				return
			}
		}
		o.removeKey(key)
	}
	o.next.HandleEvent(eventBinding)
}

// releaseArmedKeys deactivates the bindings of all armed one-shot keys by sending their key release.
func (o *OneShotHandler) releaseArmedKeys() {
	for _, k := range slices.Clone(o.keys) {
		if k.state == OneShotStateArmed {
			o.releaseKey(k)
		}
	}
}

// releaseKey deactivates the binding of the given one-shot key by sending its key release.
func (o *OneShotHandler) releaseKey(key *oneShotKey) {
	log.Debugf("OneShotHandler: releasing one-shot key %v", key.code)
	o.removeKey(key)
	o.next.HandleEvent(EventBinding{
		Event: keyboard.Event{Code: key.code, IsPress: false, Time: time.Now()},
	})
}

func (o *OneShotHandler) getKey(code uint16) *oneShotKey {
	for _, k := range o.keys {
		if k.code == code {
			return k
		}
	}
	return nil
}

func (o *OneShotHandler) removeKey(key *oneShotKey) {
	o.keys = slices.DeleteFunc(o.keys, func(k *oneShotKey) bool { return k == key })
}

// isModifierBinding returns true if the given binding presses only modifier keys.
func isModifierBinding(binding config.Binding) bool {
	keyBinding, ok := binding.(config.KeyBinding)
	if !ok {
		return false
	}
	return !slices.ContainsFunc(keyBinding.KeyCombo, func(c uint16) bool { return !config.IsModifierKey(c) })
}

// isUnboundKey returns true if the given binding of a key press does not define an own action for the key, e.g. if
// the one-shot key is pressed again while its one-shot layer is active, which does not map the key.
func isUnboundKey(binding config.Binding, code uint16) bool {
	switch b := binding.(type) {
	case nil, config.NopBinding:
		return true
	case config.KeyBinding:
		return slices.Equal(b.KeyCombo, []uint16{code}) || slices.Contains(b.KeyCombo, config.WildcardKey)
	}
	return false
}
