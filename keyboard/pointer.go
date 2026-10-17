package keyboard

import (
	evdev "github.com/gvalkov/golang-evdev"
)

// not defined in golang-evdev
const (
	relWheelHiRes  = 0x0b
	relHWheelHiRes = 0x0c
	btnTask        = 0x117
)

// PointerForwarder receives the pointer events of grabbed devices, so that devices which are a keyboard and a
// pointing device at the same time (e.g. a keyboard with a built-in touchpad) can still be used as a pointer.
type PointerForwarder interface {
	ForwardMove(x, y int32)
	// ForwardScroll scrolls by delta wheel clicks and deltaHighRes high-resolution steps (120 per wheel click).
	ForwardScroll(horizontal bool, delta, deltaHighRes int32)
	// ForwardButton presses or releases the mouse button with the given code (BTN_LEFT to BTN_TASK).
	ForwardButton(code uint16, pressed bool)
}

// pointerFrame accumulates the pointer events between two SYN_REPORT events, so that they can be forwarded together.
type pointerFrame struct {
	x, y                    int32
	wheel, hWheel           int32
	wheelHiRes, hWheelHiRes int32
	hasWheelHiRes           bool
	hasHWheelHiRes          bool
}

// handlePointerEvent forwards the given event if it is a pointer event and returns whether it was one.
func (d *Device) handlePointerEvent(event evdev.InputEvent) bool {
	switch event.Type {
	case evdev.EV_REL:
		f := &d.pointerFrame
		switch event.Code {
		case evdev.REL_X:
			f.x += event.Value
		case evdev.REL_Y:
			f.y += event.Value
		case evdev.REL_WHEEL:
			f.wheel += event.Value
		case evdev.REL_HWHEEL:
			f.hWheel += event.Value
		case relWheelHiRes:
			f.wheelHiRes += event.Value
			f.hasWheelHiRes = true
		case relHWheelHiRes:
			f.hWheelHiRes += event.Value
			f.hasHWheelHiRes = true
		}
		return true
	case evdev.EV_KEY:
		if event.Code < evdev.BTN_LEFT || event.Code > btnTask {
			return false
		}
		if event.Value == 0 || event.Value == 1 {
			d.pointerForwarder.ForwardButton(event.Code, event.Value == 1)
		}
		return true
	case evdev.EV_SYN:
		switch event.Code {
		case evdev.SYN_REPORT:
			d.flushPointerFrame()
		case evdev.SYN_DROPPED:
			// the kernel buffer overflowed, so the frame is incomplete
			d.pointerFrame = pointerFrame{}
		}
		return true
	}
	return false
}

func (d *Device) flushPointerFrame() {
	f := d.pointerFrame
	d.pointerFrame = pointerFrame{}

	if f.x != 0 || f.y != 0 {
		d.pointerForwarder.ForwardMove(f.x, f.y)
	}
	// the virtual mouse supports high-resolution scrolling, which makes libinput ignore the legacy wheel events,
	// so they have to be added if the device does not send them itself
	if !f.hasWheelHiRes {
		f.wheelHiRes = f.wheel * 120
	}
	if !f.hasHWheelHiRes {
		f.hWheelHiRes = f.hWheel * 120
	}
	if f.wheel != 0 || f.wheelHiRes != 0 {
		d.pointerForwarder.ForwardScroll(false, f.wheel, f.wheelHiRes)
	}
	if f.hWheel != 0 || f.hWheelHiRes != 0 {
		d.pointerForwarder.ForwardScroll(true, f.hWheel, f.hWheelHiRes)
	}
}
