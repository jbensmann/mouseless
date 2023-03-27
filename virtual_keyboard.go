package main

import (
	"github.com/bendahl/uinput"
	log "github.com/sirupsen/logrus"
)

type VirtualKeyboard struct {
	uinputKeyboard   uinput.Keyboard
	isPressed        map[uint16]bool
	pressedModifiers map[uint16]bool
	triggeredKeys    map[uint16][]uint16
}

func NewVirtualKeyboard() (*VirtualKeyboard, error) {
	var err error
	v := VirtualKeyboard{
		isPressed:        make(map[uint16]bool),
		pressedModifiers: make(map[uint16]bool),
		triggeredKeys:    make(map[uint16][]uint16),
	}
	v.uinputKeyboard, err = uinput.CreateKeyboard("/dev/uinput", []byte("mouseless"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *VirtualKeyboard) PressKeys(triggeredByKey uint16, codes []uint16) {
	v.triggeredKeys[triggeredByKey] = append(v.triggeredKeys[triggeredByKey], codes...)
	// release previous modifiers
	for c := range v.pressedModifiers {
		v.releaseKey(c)
	}
	for i, c := range codes {
		log.Debugf("Keyboard: pressing %v", c)
		err := v.uinputKeyboard.KeyDown(int(c))
		if err != nil {
			log.Warnf("Keyboard: failed to press the key %v: %v", c, err)
		}
		v.isPressed[c] = true
		if i < len(codes)-1 {
			v.pressedModifiers[c] = true
		}
	}
}

func (v *VirtualKeyboard) releaseKey(code uint16) {
	log.Debugf("Keyboard: releasing %v", code)
	err := v.uinputKeyboard.KeyUp(int(code))
	if err != nil {
		log.Warnf("Keyboard: failed to release the key %v: %v", code, err)
	}
	delete(v.isPressed, code)
	delete(v.pressedModifiers, code)
}

func (v *VirtualKeyboard) OriginalKeyUp(code uint16) {
	if codes, ok := v.triggeredKeys[code]; ok {
		for _, c := range codes {
			if pressed, ok := v.isPressed[c]; ok && pressed {
				v.releaseKey(c)
			}
		}
		delete(v.triggeredKeys, code)
	}
}

func (v *VirtualKeyboard) Close() {
	v.uinputKeyboard.Close()
}
