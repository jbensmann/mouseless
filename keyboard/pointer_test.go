package keyboard

import (
	"fmt"
	"reflect"
	"testing"

	evdev "github.com/gvalkov/golang-evdev"
)

type pointerForwarderMock struct {
	calls []string
}

func (p *pointerForwarderMock) ForwardMove(x, y int32) {
	p.calls = append(p.calls, fmt.Sprintf("move %d %d", x, y))
}

func (p *pointerForwarderMock) ForwardScroll(horizontal bool, delta, deltaHighRes int32) {
	p.calls = append(p.calls, fmt.Sprintf("scroll %v %d %d", horizontal, delta, deltaHighRes))
}

func (p *pointerForwarderMock) ForwardButton(code uint16, pressed bool) {
	p.calls = append(p.calls, fmt.Sprintf("button %#x %v", code, pressed))
}

func ev(eventType uint16, code uint16, value int32) evdev.InputEvent {
	return evdev.InputEvent{Type: eventType, Code: code, Value: value}
}

func syn() evdev.InputEvent {
	return ev(evdev.EV_SYN, evdev.SYN_REPORT, 0)
}

func TestHandlePointerEvent(t *testing.T) {
	tests := []struct {
		name    string
		events  []evdev.InputEvent
		handled []bool
		calls   []string
	}{
		{
			name:    "movement is forwarded once per frame",
			events:  []evdev.InputEvent{ev(evdev.EV_REL, evdev.REL_X, 3), ev(evdev.EV_REL, evdev.REL_Y, -2), syn()},
			handled: []bool{true, true, true},
			calls:   []string{"move 3 -2"},
		},
		{
			name: "movement across frames",
			events: []evdev.InputEvent{
				ev(evdev.EV_REL, evdev.REL_X, 1), syn(),
				ev(evdev.EV_REL, evdev.REL_Y, 4), syn(),
			},
			handled: []bool{true, true, true, true},
			calls:   []string{"move 1 0", "move 0 4"},
		},
		{
			name:    "legacy wheel adds high-resolution steps",
			events:  []evdev.InputEvent{ev(evdev.EV_REL, evdev.REL_WHEEL, -1), syn()},
			handled: []bool{true, true},
			calls:   []string{"scroll false -1 -120"},
		},
		{
			name: "high-resolution wheel is forwarded as is",
			events: []evdev.InputEvent{
				ev(evdev.EV_REL, relHWheelHiRes, 30), syn(),
				ev(evdev.EV_REL, evdev.REL_HWHEEL, 1), ev(evdev.EV_REL, relHWheelHiRes, 90), syn(),
			},
			handled: []bool{true, true, true, true, true},
			calls:   []string{"scroll true 0 30", "scroll true 1 90"},
		},
		{
			name: "mouse buttons are forwarded, keys and repeats are not",
			events: []evdev.InputEvent{
				ev(evdev.EV_KEY, evdev.BTN_LEFT, 1), ev(evdev.EV_KEY, evdev.KEY_A, 1),
				ev(evdev.EV_KEY, evdev.BTN_LEFT, 2), ev(evdev.EV_KEY, evdev.BTN_LEFT, 0), syn(),
				ev(evdev.EV_KEY, evdev.BTN_SIDE, 1), ev(evdev.EV_KEY, btnTask, 1),
				ev(evdev.EV_KEY, btnTask+1, 1), ev(evdev.EV_KEY, evdev.BTN_MOUSE-1, 1),
			},
			handled: []bool{true, false, true, true, true, true, true, false, false},
			calls:   []string{"button 0x110 true", "button 0x110 false", "button 0x113 true", "button 0x117 true"},
		},
		{
			name: "dropped frame is discarded",
			events: []evdev.InputEvent{
				ev(evdev.EV_REL, evdev.REL_X, 5), ev(evdev.EV_SYN, evdev.SYN_DROPPED, 0), syn(),
			},
			handled: []bool{true, true, true},
			calls:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			forwarder := &pointerForwarderMock{}
			d := NewKeyboardDevice(nil, nil, forwarder)
			var handled []bool
			for _, event := range test.events {
				handled = append(handled, d.handlePointerEvent(event))
			}
			if !reflect.DeepEqual(handled, test.handled) {
				t.Errorf("handled: got %v, want %v", handled, test.handled)
			}
			if !reflect.DeepEqual(forwarder.calls, test.calls) {
				t.Errorf("calls: got %v, want %v", forwarder.calls, test.calls)
			}
		})
	}
}
