package virtual

import (
	"github.com/jbensmann/mouseless/config"
	"math"
	"time"

	"github.com/bendahl/uinput"
	log "github.com/sirupsen/logrus"
)

type Direction struct {
	x float64
	y float64
}

func (d *Direction) Add(d2 Direction) {
	d.x += d2.x
	d.y += d2.y
}

type VirtualMouse struct {
	uinputMouse uinput.Mouse

	mouseLoopInterval      time.Duration
	baseMouseSpeed         float64
	baseScrollSpeed        float64
	startMouseSpeed        float64
	mouseAccelerationTime  float64
	mouseDecelerationTime  float64
	mouseAccelerationCurve float64
	mouseDecelerationCurve float64

	isPressed map[config.MouseButton]bool

	triggeredKeys map[uint16]config.MouseButton
	moveByKeys    map[uint16]Direction
	scrollByKeys  map[uint16]Direction
	speedByKeys   map[uint16]float64

	targetVelocity Direction
	velocity       Direction
	moveFraction   Direction
	scrollFraction Direction
}

func NewVirtualMouse() (*VirtualMouse, error) {
	var err error
	v := VirtualMouse{
		mouseLoopInterval: 20 * time.Millisecond,
		isPressed:         make(map[config.MouseButton]bool),
		triggeredKeys:     make(map[uint16]config.MouseButton),
		scrollFraction:    Direction{},
	}
	v.uinputMouse, err = uinput.CreateMouse("/dev/uinput", []byte("mouseless"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *VirtualMouse) ButtonPress(triggeredByKey uint16, button config.MouseButton) {
	var err error
	v.triggeredKeys[triggeredByKey] = button
	v.isPressed[button] = true
	log.Debug("Mouse: pressing %v", button)
	if button == config.ButtonLeft {
		err = v.uinputMouse.LeftPress()
	} else if button == config.ButtonMiddle {
		err = v.uinputMouse.MiddlePress()
	} else if button == config.ButtonRight {
		err = v.uinputMouse.RightPress()
	} else {
		log.Warnf("Mouse: unknown button: %v", button)
	}
	if err != nil {
		log.Warnf("Mouse: button press failed: %v", err)
	}
}

func (v *VirtualMouse) OriginalKeyUp(code uint16) {
	delete(v.moveByKeys, code)
	delete(v.scrollByKeys, code)
	delete(v.speedByKeys, code)

	if button, ok := v.triggeredKeys[code]; ok {
		if pressed, ok := v.isPressed[button]; ok && pressed {
			var err error
			log.Debugf("Mouse: releasing %v", button)
			if button == config.ButtonLeft {
				err = v.uinputMouse.LeftRelease()
			} else if button == config.ButtonMiddle {
				err = v.uinputMouse.MiddleRelease()
			} else if button == config.ButtonRight {
				err = v.uinputMouse.RightRelease()
			} else {
				log.Warnf("Mouse: unknown button: %v", button)
			}
			if err != nil {
				log.Warnf("Mouse: button release failed: %v", err)
			}
			delete(v.isPressed, button)
		}
		delete(v.triggeredKeys, code)
	}
}

func (v *VirtualMouse) Scroll(x float64, y float64) {
	v.scrollFraction.x += x
	v.scrollFraction.y += y
	// move only the integer part
	var xInt = int32(v.scrollFraction.x)
	var yInt = int32(v.scrollFraction.y)
	v.scrollFraction.x -= float64(xInt)
	v.scrollFraction.y -= float64(yInt)
	if xInt != 0 {
		log.Debugf("Mouse: scroll horizontal: %v", xInt)
		err := v.uinputMouse.Wheel(true, xInt)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
	if yInt != 0 {
		log.Debugf("Mouse: scroll vertical: %v", yInt)
		err := v.uinputMouse.Wheel(false, -yInt)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
}

func (v *VirtualMouse) ChangeMoveSpeed(triggeredByKey uint16, x float64, y float64) {
	v.moveByKeys[triggeredByKey] = Direction{x, y}
}

func (v *VirtualMouse) ChangeScrollSpeed(triggeredByKey uint16, x float64, y float64) {
	v.scrollByKeys[triggeredByKey] = Direction{x, y}
}

func (v *VirtualMouse) AddSpeedFactor(triggeredByKey uint16, speedFactor float64) {
	v.speedByKeys[triggeredByKey] = speedFactor
}

func moveTowards(
	current float64,
	target float64,
	max float64,
	start float64,
	accelerationCurve float64,
	accelerationStep float64,
	decelerationCurve float64,
	decelerationStep float64,
) float64 {
	if target < 0 || (target == 0 && current < 0) {
		return -moveTowards(-current, -target, max, start, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep)
	}
	if current <= 0 && target > 0 {
		current = start
	}
	if current < target {
		t := math.Pow(current/max, 1/accelerationCurve) + accelerationStep
		return math.Min(target, target*math.Pow(t, accelerationCurve))
	} else {
		t := math.Pow(current/max, 1/decelerationCurve) - decelerationStep
		if t <= 0.0 {
			return target
		}
		return math.Max(target, max*(math.Pow(t, decelerationCurve)))
	}
}

func (v *VirtualMouse) Move(
	x float64, y float64, startMouseSpeed float64, maxMouseSpeed float64,
	accelerationCurve float64, accelerationStep float64,
	decelerationCurve float64, decelerationStep float64,
	speedFactor float64,
) {
	v.velocity.x = moveTowards(v.velocity.x, x, maxMouseSpeed, startMouseSpeed, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep)
	v.velocity.y = moveTowards(v.velocity.y, y, maxMouseSpeed, startMouseSpeed, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep)
	v.moveFraction.x += v.velocity.x * speedFactor
	v.moveFraction.y += v.velocity.y * speedFactor
	// move only the integer part
	var xInt = int32(v.moveFraction.x)
	var yInt = int32(v.moveFraction.y)
	v.moveFraction.x -= float64(xInt)
	v.moveFraction.y -= float64(yInt)
	if xInt != 0 || yInt != 0 {
		log.Debugf("Mouse: move %v %v", xInt, yInt)
		err := v.uinputMouse.Move(xInt, yInt)
		if err != nil {
			log.Warnf("Mouse: move failed: %v", err)
		}
	}
}

func (v *VirtualMouse) IsMoving() bool {
	return v.velocity.x != 0 || v.velocity.y != 0
}

func (v *VirtualMouse) Close() {
	v.uinputMouse.Close()
}

func (v *VirtualMouse) mainLoop() {
	mouseTimer := time.NewTimer(math.MaxInt64)
	lastUpdate := time.Now()

	for {
		select {
		case <-mouseTimer.C:
		}

		// how much time has passed?
		now := time.Now()
		updateDuration := now.Sub(lastUpdate)
		lastUpdate = now

		// handle mouse movement and scrolling
		var move Direction
		var scroll Direction
		speedFactor := 1.0

		for _, dir := range v.moveByKeys {
			move.Add(dir)
		}
		for _, dir := range v.scrollByKeys {
			scroll.Add(dir)
		}
		for _, speed := range v.speedByKeys {
			speedFactor *= speed
		}

		if move.x != 0 || move.y != 0 || scroll.x != 0 || scroll.y != 0 || v.IsMoving() {
			tickTime := updateDuration.Seconds()
			moveSpeed := v.baseMouseSpeed * tickTime
			scrollSpeed := v.baseScrollSpeed * tickTime
			accelerationStep := tickTime * 1000 / v.mouseAccelerationTime
			decelerationStep := tickTime * 1000 / v.mouseDecelerationTime
			v.Scroll(scroll.x*scrollSpeed*speedFactor, scroll.y*scrollSpeed*speedFactor)
			v.Move(
				move.x*moveSpeed, move.y*moveSpeed, v.startMouseSpeed*tickTime,
				v.baseMouseSpeed*tickTime,
				v.mouseAccelerationCurve,
				accelerationStep,
				v.mouseDecelerationCurve,
				decelerationStep,
				speedFactor,
			)
			mouseTimer = time.NewTimer(v.mouseLoopInterval)
		} else {
			mouseTimer = time.NewTimer(math.MaxInt64)
		}
	}
}
