package virtual

import (
	"math"
	"sync"
	"time"

	"github.com/jbensmann/mouseless/config"

	"github.com/jbensmann/uinput"
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

	isRunning             bool
	velocity              Vector
	moveFraction          Vector
	scrollFraction        Vector
	scrollFractionHighRes Vector

	lock                   sync.Mutex
	mouseLoopTimer         *time.Timer
	mouseMoveEventsChannel chan struct{}
}

func NewMouse(conf *config.Config) (*Mouse, error) {
	var err error
	v := Mouse{
		isButtonPressed:        make(map[config.MouseButton]bool),
		buttonsByKeys:          make(map[uint16]config.MouseButton),
		moveByKeys:             make(map[uint16]Vector),
		scrollByKeys:           make(map[uint16]Vector),
		speedByKeys:            make(map[uint16]float64),
		velocity:               Vector{},
		moveFraction:           Vector{},
		scrollFraction:         Vector{},
		scrollFractionHighRes:  Vector{},
		mouseMoveEventsChannel: make(chan struct{}, 1),
	}
	v.SetConfig(conf)
	v.uinputMouse, err = uinput.CreateMouse("/dev/uinput", []byte("mouseless"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// SetConfig updates the relevant parameters from the config file.
func (m *Mouse) SetConfig(conf *config.Config) {
	m.mouseLoopInterval = time.Duration(conf.MouseLoopInterval) * time.Millisecond
	m.baseMouseSpeed = conf.BaseMouseSpeed
	m.baseScrollSpeed = conf.BaseScrollSpeed
	m.startMouseSpeed = conf.StartMouseSpeed
	m.mouseAccelerationTime = conf.MouseAccelerationTime
	m.mouseDecelerationTime = conf.MouseDecelerationTime
	m.mouseAccelerationCurve = conf.MouseAccelerationCurve
	m.mouseDecelerationCurve = conf.MouseDecelerationCurve
}

func (m *Mouse) StartLoop() {
	m.isRunning = true
	go m.mainLoop()
}

func (m *Mouse) ButtonPress(triggeredByKey uint16, button config.MouseButton) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var err error
	m.buttonsByKeys[triggeredByKey] = button
	m.isButtonPressed[button] = true
	log.Debugf("Mouse: pressing %v", button)
	if button == config.ButtonLeft {
		err = m.uinputMouse.LeftPress()
	} else if button == config.ButtonMiddle {
		err = m.uinputMouse.MiddlePress()
	} else if button == config.ButtonRight {
		err = m.uinputMouse.RightPress()
	} else {
		log.Warnf("Mouse: unknown button: %v", button)
	}
	if err != nil {
		log.Warnf("Mouse: button press failed: %v", err)
	}
}

func (m *Mouse) ChangeMoveSpeed(triggeredByKey uint16, x float64, y float64) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.moveByKeys[triggeredByKey] = Vector{x, y}
	m.mouseMoveChange()
}

func (m *Mouse) ChangeScrollSpeed(triggeredByKey uint16, x float64, y float64) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.scrollByKeys[triggeredByKey] = Vector{x, y}
	m.mouseMoveChange()
}

func (m *Mouse) AddSpeedFactor(triggeredByKey uint16, speedFactor float64) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.speedByKeys[triggeredByKey] = speedFactor
	m.mouseMoveChange()
}

func (m *Mouse) OriginalKeyUp(code uint16) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.moveByKeys, code)
	delete(m.scrollByKeys, code)
	delete(m.speedByKeys, code)

	if button, ok := m.buttonsByKeys[code]; ok {
		if pressed, ok := m.isButtonPressed[button]; ok && pressed {
			var err error
			log.Debugf("Mouse: releasing %v", button)
			if button == config.ButtonLeft {
				err = m.uinputMouse.LeftRelease()
			} else if button == config.ButtonMiddle {
				err = m.uinputMouse.MiddleRelease()
			} else if button == config.ButtonRight {
				err = m.uinputMouse.RightRelease()
			} else {
				log.Warnf("Mouse: unknown button: %v", button)
			}
			if err != nil {
				log.Warnf("Mouse: button release failed: %v", err)
			}
			delete(m.isButtonPressed, button)
		}
		delete(m.buttonsByKeys, code)
	}
}

func (m *Mouse) Close() {
	m.isRunning = false

	m.lock.Lock()
	defer m.lock.Unlock()

	_ = m.uinputMouse.Close()
}

func (m *Mouse) mainLoop() {
	lastUpdate := time.Now()

	for m.isRunning {
		if m.mouseLoopTimer != nil {
			<-m.mouseLoopTimer.C
		} else {
			// wait for an incoming mouse movement event
			<-m.mouseMoveEventsChannel
			// set lastUpdate to the past so that the mouse starts moving immediately
			lastUpdate = time.Now().Add(-m.mouseLoopInterval)
		}

		// how much time has passed?
		now := time.Now()
		updateDuration := now.Sub(lastUpdate)
		lastUpdate = now

		m.moveAndScroll(updateDuration)
	}
}

func (m *Mouse) moveAndScroll(updateDuration time.Duration) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var move Vector
	var scroll Vector
	speedFactor := 1.0

	for _, dir := range m.moveByKeys {
		move.Add(dir)
	}
	for _, dir := range m.scrollByKeys {
		scroll.Add(dir)
	}
	for _, speed := range m.speedByKeys {
		speedFactor *= speed
	}

	if len(m.moveByKeys) > 0 || len(m.scrollByKeys) > 0 || m.isMoving() {
		tickTime := updateDuration.Seconds()
		moveSpeed := m.baseMouseSpeed * tickTime
		scrollSpeed := m.baseScrollSpeed * tickTime
		accelerationStep := tickTime * 1000 / m.mouseAccelerationTime
		decelerationStep := tickTime * 1000 / m.mouseDecelerationTime
		m.scroll(scroll.x*scrollSpeed*speedFactor, scroll.y*scrollSpeed*speedFactor)
		m.move(
			move.x*moveSpeed, move.y*moveSpeed, m.startMouseSpeed*tickTime,
			m.baseMouseSpeed*tickTime,
			m.mouseAccelerationCurve,
			accelerationStep,
			m.mouseDecelerationCurve,
			decelerationStep,
			speedFactor,
		)
		m.mouseLoopTimer = time.NewTimer(m.mouseLoopInterval)
	} else {
		m.mouseLoopTimer = nil
	}
}

// mouseMoveChange sends a signal to the main loop that the mouse movement has changed.
func (m *Mouse) mouseMoveChange() {
	select {
	case m.mouseMoveEventsChannel <- struct{}{}:
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

func (m *Mouse) move(
	x float64, y float64, startMouseSpeed float64, maxMouseSpeed float64,
	accelerationCurve float64, accelerationStep float64,
	decelerationCurve float64, decelerationStep float64,
	speedFactor float64,
) {
	m.velocity.x = moveTowards(m.velocity.x, x, maxMouseSpeed, startMouseSpeed, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep)
	m.velocity.y = moveTowards(m.velocity.y, y, maxMouseSpeed, startMouseSpeed, accelerationCurve, accelerationStep, decelerationCurve, decelerationStep)
	m.moveFraction.x += m.velocity.x * speedFactor
	m.moveFraction.y += m.velocity.y * speedFactor
	// move only the integer part
	var xInt = int32(m.moveFraction.x)
	var yInt = int32(m.moveFraction.y)
	m.moveFraction.x -= float64(xInt)
	m.moveFraction.y -= float64(yInt)
	if xInt != 0 || yInt != 0 {
		log.Debugf("Mouse: move %v %v", xInt, yInt)
		err := m.uinputMouse.Move(xInt, yInt)
		if err != nil {
			log.Warnf("Mouse: move failed: %v", err)
		}
	}
}

func (m *Mouse) scroll(x float64, y float64) {
	// send discrete scroll events for legacy (one wheel click is one step)
	xSteps, ySteps := getScrollingSteps(&m.scrollFractionHighRes, x, y, 120)
	if xSteps != 0 {
		log.Debugf("Mouse: scroll horizontal highRes: %v", xSteps)
		err := m.uinputMouse.WheelHighRes(true, xSteps)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
	if ySteps != 0 {
		log.Debugf("Mouse: scroll vertical highRes: %v", ySteps)
		err := m.uinputMouse.WheelHighRes(false, -ySteps)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
	// with high-resolution scrolling events, one wheel click corresponds to 120
	// high-resolution steps
	xSteps, ySteps = getScrollingSteps(&m.scrollFraction, x, y, 1)
	if xSteps != 0 {
		log.Debugf("Mouse: scroll horizontal: %v", xSteps)
		err := m.uinputMouse.Wheel(true, xSteps)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
	if ySteps != 0 {
		log.Debugf("Mouse: scroll vertical: %v", ySteps)
		err := m.uinputMouse.Wheel(false, -ySteps)
		if err != nil {
			log.Warnf("Mouse: scroll failed: %v", err)
		}
	}
}

// getScrollingSteps calculates discrete scroll steps from fractional input.
// scrollFraction accumulates leftover fractional scrolls.
// x and y specify the scroll deltas in each direction.
// stepCount determines the resolution for conversion to discrete steps (e.g., a
// value of 10 would mean that x=1.0 or y=1.0 would result in 10 scroll steps).
func getScrollingSteps(scrollFraction *Vector, x float64, y float64, stepCount int32) (int32, int32) {
	// when the direction changes, start at 0 again
	if x > 0 {
		scrollFraction.x = max(scrollFraction.x, 0)
	} else if x < 0 {
		scrollFraction.x = min(scrollFraction.x, 0)
	}
	if y > 0 {
		scrollFraction.y = max(scrollFraction.y, 0)
	} else if y < 0 {
		scrollFraction.y = min(scrollFraction.y, 0)
	}
	scrollFraction.x += x
	scrollFraction.y += y
	var xSteps = int32(scrollFraction.x * float64(stepCount))
	var ySteps = int32(scrollFraction.y * float64(stepCount))
	scrollFraction.x -= float64(xSteps) / float64(stepCount)
	scrollFraction.y -= float64(ySteps) / float64(stepCount)
	return xSteps, ySteps
}

func (m *Mouse) isMoving() bool {
	return m.velocity.x != 0 || m.velocity.y != 0
}
