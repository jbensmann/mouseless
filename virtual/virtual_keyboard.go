package virtual

import (
	"github.com/jbensmann/mouseless/config"
	"github.com/jbensmann/uinput"
	log "github.com/sirupsen/logrus"
)

type Keyboard struct {
	uinputKeyboard   uinput.Keyboard
	isPressed        map[uint16]bool
	pressedModifiers map[uint16]bool
	triggeredKeys    map[uint16][]uint16
}

func NewKeyboard() (*Keyboard, error) {
	var err error
	v := Keyboard{
		isPressed:        make(map[uint16]bool),
		pressedModifiers: make(map[uint16]bool),
		triggeredKeys:    make(map[uint16][]uint16),
	}
	v.uinputKeyboard, err = uinput.CreateKeyboard("/dev/uinput", []byte("mouseless keyboard"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// PressKeys presses the given keys and releases them automatically when the given trigger key is released.
func (v *Keyboard) PressKeys(triggeredByKey uint16, codes []uint16) {
	v.triggeredKeys[triggeredByKey] = append(v.triggeredKeys[triggeredByKey], codes...)
	// release previous modifiers
	for c := range v.pressedModifiers {
		v.releaseKey(c)
	}
	for i, c := range codes {
		v.pressKey(c)
		if i < len(codes)-1 {
			v.pressedModifiers[c] = true
		}
	}
}

// PressKeyManually can be used to press a key without automatic release, which must be done by calling ReleaseKeyManually.
func (v *Keyboard) PressKeyManually(code uint16) {
	v.pressKey(code)
}

// ReleaseKeyManually must be called eventually to release a key that was pressed via PressKeyManually.
func (v *Keyboard) ReleaseKeyManually(code uint16) {
	v.releaseKey(code)
}

func (v *Keyboard) pressKey(code uint16) {
	alias, _ := config.GetKeyAlias(code)
	log.Debugf("Keyboard: pressing %v (%v)", alias, code)
	err := v.uinputKeyboard.KeyDown(int(code))
	if err != nil {
		log.Warnf("Keyboard: failed to press the key %v: %v", code, err)
	}
	v.isPressed[code] = true
}

func (v *Keyboard) releaseKey(code uint16) {
	alias, _ := config.GetKeyAlias(code)
	log.Debugf("Keyboard: releasing %v (%v)", alias, code)
	err := v.uinputKeyboard.KeyUp(int(code))
	if err != nil {
		log.Warnf("Keyboard: failed to release the key %v: %v", code, err)
	}
	delete(v.isPressed, code)
	delete(v.pressedModifiers, code)
}

func (v *Keyboard) OriginalKeyUp(code uint16) {
	if codes, ok := v.triggeredKeys[code]; ok {
		for _, c := range codes {
			if pressed, ok := v.isPressed[c]; ok && pressed {
				v.releaseKey(c)
			}
		}
		delete(v.triggeredKeys, code)
	}
}

func (v *Keyboard) Close() {
	v.uinputKeyboard.Close()
}
