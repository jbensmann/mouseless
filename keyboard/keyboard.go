package keyboard

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jbensmann/mouseless/config"

	evdev "github.com/gvalkov/golang-evdev"
	log "github.com/sirupsen/logrus"
)

type Event struct {
	Code    uint16
	IsPress bool
	Time    time.Time
}

type DeviceState int

const (
	StateNotOpen DeviceState = iota
	StateOpenFailed
	StateOpen
)

type Device struct {
	device        *evdev.InputDevice
	state         DeviceState
	lastOpenError string
	eventChan     chan<- Event
}

func NewKeyboardDevice(device *evdev.InputDevice, eventChan chan<- Event) *Device {
	k := Device{
		device:    device,
		state:     StateNotOpen,
		eventChan: eventChan,
	}
	return &k
}

func (d *Device) GrabDevice() error {
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
