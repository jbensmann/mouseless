package keyboard

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"syscall"
	"time"
	"unsafe"

	"github.com/jbensmann/mouseless/config"

	evdev "github.com/gvalkov/golang-evdev"
	log "github.com/sirupsen/logrus"
)

type Event struct {
	Code    uint16
	IsPress bool
	Time    time.Time
}

// grabWaitTimeout is the maximum time to wait for all keys to be released before grabbing a device.
const grabWaitTimeout = 5 * time.Second

type DeviceState int

const (
	StateNotOpen DeviceState = iota
	StateOpenFailed
	StateOpen
)

type Device struct {
	device           *evdev.InputDevice
	state            DeviceState
	lastOpenError    string
	eventChan        chan<- Event
	pointerForwarder PointerForwarder
	pointerFrame     pointerFrame
}

func NewKeyboardDevice(device *evdev.InputDevice, eventChan chan<- Event, pointerForwarder PointerForwarder) *Device {
	k := Device{
		device:           device,
		state:            StateNotOpen,
		eventChan:        eventChan,
		pointerForwarder: pointerForwarder,
	}
	return &k
}

func (d *Device) GrabDevice() error {
	d.waitForKeysReleased()
	err := d.device.Grab()
	if err != nil {
		d.state = StateOpenFailed
		return err
	}
	log.Debugf("Grabbed device: %s", d.device)

	d.state = StateOpen
	go d.readKeyboard()
	return nil
}

// waitForKeysReleased waits until no key of the device is pressed anymore (or a timeout is reached).
// If a key is pressed while grabbing the device, its release is only sent to us, so everyone else
// (e.g. the compositor) would consider the key to be pressed forever.
func (d *Device) waitForKeysReleased() {
	deadline := time.Now().Add(grabWaitTimeout)
	logged := false
	for {
		pressed, err := d.anyKeyPressed()
		if err != nil {
			log.Warnf("Failed to get the key state of device %s: %v", d, err)
			return
		}
		if !pressed {
			return
		}
		if time.Now().After(deadline) {
			log.Warnf("Keys of device %s are still pressed, grabbing it anyway", d)
			return
		}
		if !logged {
			log.Debugf("Waiting for all keys of device %s to be released", d)
			logged = true
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// anyKeyPressed returns whether any key of the device is currently pressed.
func (d *Device) anyKeyPressed() (bool, error) {
	// EVIOCGKEY is defined with a buffer size of evdev.MAX_NAME_SIZE, which is more than enough for KEY_MAX bits
	var state [evdev.MAX_NAME_SIZE]byte
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, d.device.File.Fd(), uintptr(evdev.EVIOCGKEY), uintptr(unsafe.Pointer(&state)),
	)
	if errno != 0 {
		return false, errno
	}
	return slices.ContainsFunc(state[:], func(b byte) bool { return b != 0 }), nil
}

// readKeyboard reads from the device in an infinite loop.
// The device has to be opened, and if it disconnects in between this method returns and sets the state to not open.
func (d *Device) readKeyboard() {
	var events []evdev.InputEvent
	var err error
	for {
		if d.state != StateOpen {
			return
		}
		events, err = d.device.Read()
		if err != nil {
			// don't log a warning if the device path does not exist anymore,
			// which probably means that the keyboard has been unplugged
			var pathErr *fs.PathError
			if !errors.As(err, &pathErr) {
				log.Warnf("Failed to read keyboard: %T", err)
			}
			d.state = StateNotOpen
			return
		}
		for _, event := range events {
			// the device is grabbed, so pointer events (e.g. from a built-in touchpad) have to be forwarded
			if d.handlePointerEvent(event) {
				continue
			}
			if event.Type == evdev.EV_KEY {
				if event.Value == 0 || event.Value == 1 {

					codeAlias, exists := config.GetKeyAlias(event.Code)
					if !exists {
						codeAlias = "?"
					}
					fmtString := "Pressed:  "
					if event.Value == 0 {
						fmtString = "Released: "
					}
					fmtString += "%s (%d)"
					log.Debugf(fmtString, codeAlias, event.Code)

					e := Event{
						Code:    event.Code,
						IsPress: event.Value == 1,
						Time:    time.Now(),
					}
					d.eventChan <- e
				}
			}
		}
	}
}

// String returns a string representation of the device.
func (d *Device) String() string {
	return fmt.Sprintf("%s (%s)", d.device.Fn, d.device.Name)
}

// Path returns the path of the keyboard device.
func (d *Device) Path() string {
	return d.device.Fn
}

// Name returns the name of the keyboard device.
func (d *Device) Name() string {
	return d.device.Name
}

// IsOpen returns true if the device has been opened successfully.
func (d *Device) IsOpen() bool {
	return d.state == StateOpen
}

// LastOpenError returns the last error on opening the device.
func (d *Device) LastOpenError() string {
	return d.lastOpenError
}

// Disconnected shall be called when a device has been removed.
func (d *Device) Disconnected() {
	d.state = StateNotOpen
}
