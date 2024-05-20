package virtual

import (
	"github.com/jbensmann/mouseless/config"
	"math"
	"time"

	"github.com/bendahl/uinput"
	log "github.com/sirupsen/logrus"
)

type Vector struct {
	x float64
	y float64
}

func (d *Vector) Add(d2 Vector) {
	d.x += d2.x
	d.y += d2.y
}

type Mouse struct {
	uinputMouse uinput.Mouse

	mouseLoopInterval      time.Duration
	baseMouseSpeed         float64
	baseScrollSpeed        float64
	startMouseSpeed        float64
	mouseAccelerationTime  float64
	mouseDecelerationTime  float64
	mouseAccelerationCurve float64
	mouseDecelerationCurve float64

	isButtonPressed map[config.MouseButton]bool

	buttonsByKeys map[uint16]config.MouseButton
	moveByKeys    map[uint16]Vector
	scrollByKeys  map[uint16]Vector
	speedByKeys   map[uint16]float64

	velocity       Vector
	moveFraction   Vector
	scrollFraction Vector

	mouseMoveChangeChannel chan struct{}
}

func NewMouse(conf *config.Config) (*Mouse, error) {
	var err error
	v := Mouse{
		mouseLoopInterval:      20 * time.Millisecond,
		baseMouseSpeed:         conf.BaseMouseSpeed,
		baseScrollSpeed:        conf.BaseScrollSpeed,
		startMouseSpeed:        conf.StartMouseSpeed,
		mouseAccelerationTime:  conf.MouseAccelerationTime,
		mouseDecelerationTime:  conf.MouseDecelerationTime,
		mouseAccelerationCurve: conf.MouseAccelerationCurve,
		mouseDecelerationCurve: conf.MouseDecelerationCurve,

		isButtonPressed:        make(map[config.MouseButton]bool),
		buttonsByKeys:          make(map[uint16]config.MouseButton),
		moveByKeys:             make(map[uint16]Vector),
		scrollByKeys:           make(map[uint16]Vector),
		speedByKeys:            make(map[uint16]float64),
		velocity:               Vector{},
		moveFraction:           Vector{},
		scrollFraction:         Vector{},
		mouseMoveChangeChannel: make(chan struct{}, 1),
	}
	v.uinputMouse, err = uinput.CreateMouse("/dev/uinput", []byte("mouseless"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (v *Mouse) StartLoop() {
	go v.mainLoop()
}

func (v *Mouse) ButtonPress(triggeredByKey uint16, button config.MouseButton) {
	var err error
	v.buttonsByKeys[triggeredByKey] = button
	v.isButtonPressed[button] = true
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

func (v *Mouse) ChangeMoveSpeed(triggeredByKey uint16, x float64, y float64) {
	v.moveByKeys[triggeredByKey] = Vector{x, y}
	v.mouseMoveChange()
}

func (v *Mouse) ChangeScrollSpeed(triggeredByKey uint16, x float64, y float64) {
	v.scrollByKeys[triggeredByKey] = Vector{x, y}
	v.mouseMoveChange()
}

func (v *Mouse) AddSpeedFactor(triggeredByKey uint16, speedFactor float64) {
	v.speedByKeys[triggeredByKey] = speedFactor
	v.mouseMoveChange()
}

func (v *Mouse) OriginalKeyUp(code uint16) {
	delete(v.moveByKeys, code)
	delete(v.scrollByKeys, code)
	delete(v.speedByKeys, code)

	if button, ok := v.buttonsByKeys[code]; ok {
		if pressed, ok := v.isButtonPressed[button]; ok && pressed {
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
			delete(v.isButtonPressed, button)
		}
		delete(v.buttonsByKeys, code)
	}
}

func (v *Mouse) Close() {
	_ = v.uinputMouse.Close()
	// todo: stop the main loop
}

func (v *Mouse) mainLoop() {
	mouseTimer := time.NewTimer(v.mouseLoopInterval)
	lastUpdate := time.Now()

	for {
		select {
		case <-mouseTimer.C:
		case <-v.mouseMoveChangeChannel:
			// todo: check if we need to check for scrolling
			if !v.isMoving() {
				lastUpdate = time.Now()
			}
		}

		// how much time has passed?
		now := time.Now()
		updateDuration := now.Sub(lastUpdate)
		lastUpdate = now

		// handle mouse movement and scrolling
		var move Vector
		var scroll Vector
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

		if move.x != 0 || move.y != 0 || scroll.x != 0 || scroll.y != 0 || v.isMoving() {
			log.Debugf("mouse move: %v, scroll: %v", move, scroll)

			tickTime := updateDuration.Seconds()
			moveSpeed := v.baseMouseSpeed * tickTime
			scrollSpeed := v.baseScrollSpeed * tickTime
			accelerationStep := tickTime * 1000 / v.mouseAccelerationTime
			decelerationStep := tickTime * 1000 / v.mouseDecelerationTime
			v.scroll(scroll.x*scrollSpeed*speedFactor, scroll.y*scrollSpeed*speedFactor)
			v.move(
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

// mouseMoveChange sends a signal to the main loop that the mouse movement has changed.
func (v *Mouse) mouseMoveChange() {
	select {
	case v.mouseMoveChangeChannel <- struct{}{}:
	default:
	}
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

func (v *Mouse) move(
	x float64, y float64, startMouseSpeed float64, maxMouseSpeed float64,
	accelerationCurve float64, accelerationStep float64,
	decelerationCurve float64, decelerationStep float64,
	speedFactor float64,
) {
	log.Debugf("move called with: x=%f, y=%f, startMouseSpeed=%f, maxMouseSpeed=%f, accelerationCurve=%f, accelerationStep=%f, decelerationCurve=%f, decelerationStep=%f, speedFactor=%f",
		x, y, startMouseSpeed, maxMouseSpeed, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep, speedFactor)

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

func (v *Mouse) scroll(x float64, y float64) {
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

func (v *Mouse) isMoving() bool {
	return v.velocity.x != 0 || v.velocity.y != 0
}
