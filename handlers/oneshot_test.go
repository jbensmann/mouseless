package handlers

import (
	"testing"
)

// chainHandler chains the given handlers, so that they can be tested together.
type chainHandler struct {
	handlers []EventHandler
}

func newChainHandler(handlers ...EventHandler) *chainHandler {
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNextHandler(handlers[i+1])
	}
	return &chainHandler{handlers: handlers}
}

func (c *chainHandler) HandleEvent(event EventBinding) {
	c.handlers[0].HandleEvent(event)
}

func (c *chainHandler) SetNextHandler(handler EventHandler) {
	c.handlers[len(c.handlers)-1].SetNextHandler(handler)
}

func (c *chainHandler) SetLayerManager(manager LayerManager) {
	for _, h := range c.handlers {
		h.SetLayerManager(manager)
	}
}

func TestOneShot(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: one-shot leftshift
    s: one-shot leftctrl
    b: one-shot toggle-layer 2
    e: one-shot toggle-layer 3
- name: 2
  bindings:
    c: x
- name: 3
  passThrough: false
  bindings:
    d: d
`
	tests := [][]string{
		{"Pc Rc", "Pc:Kc Rc"},                                                                     // unrelated keys are passed through
		{"Pa Ra Pc Rc", "Pa:Kleftshift Pc:Kc Ra Rc"},                                              // tap: active for the next key press
		{"Pa Ra Pc Rc Pd Rd", "Pa:Kleftshift Pc:Kc Ra Rc Pd:Kd Rd"},                               // only for the next key press
		{"Pa Ra Pc Pd Rc Rd", "Pa:Kleftshift Pc:Kc Ra Pd:Kd Rc Rd"},                               // rolling over to the next key
		{"Pa Pc Rc Ra", "Pa:Kleftshift Pc:Kc Rc Ra"},                                              // hold
		{"Pa Pc Ra Rc", "Pa:Kleftshift Pc:Kc Ra Rc"},                                              // hold, released before the other key
		{"Pa Ra Ps Rs Pc Rc", "Pa:Kleftshift Ps:Kleftctrl Pc:Kc Ra Rs Rc"},                        // two one-shot keys are combined
		{"Pa Ra Ps Pc Rc Rs", "Pa:Kleftshift Ps:Kleftctrl Pc:Kc Ra Rc Rs"},                        // one-shot key combined with held one-shot key
		{"Pa Ra Pleftalt Pc Rc Rleftalt", "Pa:Kleftshift Pleftalt:Kleftalt Pc:Kc Ra Rc Rleftalt"}, // modifiers don't consume it
		{"Pa Ra Pa Ra Pc Rc Pd Rd Pa Ra", "Pa:Kleftshift Pc:Kc Rc Pd:Kd Rd Ra"},                   // double tap locks it
		{"Pa Ra Pa Ra Pa Pc Rc Ra Pd Rd", "Pa:Kleftshift Pc:Kc Rc Ra Pd:Kd Rd"},                   // held while locked unlocks it
		{"Pa Ra Pa Pc Rc Ra Pd Rd", "Pa:Kleftshift Pc:Kc Rc Ra Pd:Kd Rd"},                         // held while armed
		{"Pb Rb Pc Rc Pc Rc", "Pb:L2 Pc:Kx Rb Rc Pc:Kc Rc"},                                       // one-shot layer
		{"Pb Pc Rc Rb Pc Rc", "Pb:L2 Pc:Kx Rc Rb Pc:Kc Rc"},                                       // held layer
		{"Pb Rb Pb Rb Pc Rc Pb Rb", "Pb:L2 Pc:Kx Rc Rb"},                                          // double tap locks the layer
		{"Pe Re Pc Rc Pc Rc", "Pe:L3 Pc Re Rc Pc:Kc Rc"},                                          // unbound keys consume it as well
	}
	handler := func() EventHandler { return newChainHandler(NewDefaultHandler(), NewOneShotHandler()) }
	testHandler(t, handler, configStr, tests)
}

func TestOneShotWithinTapHold(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold one-shot leftshift ; toggle-layer 2 ; 10
- name: 2
  bindings:
    c: x
`
	tests := [][]string{
		{"Pa Ra Pc Rc", "Pa:Kleftshift Pc:Kc Ra Rc"},                   // tap
		{"Pa 15 Pc Rc Ra", "Pa:L2 Pc:Kx Rc Ra"},                        // hold
		{"Pa Ra Pa Ra Pc Rc Pc Rc", "Pa:Kleftshift Pc:Kc Rc Pc:Kc Rc"}, // double tap
		{"Pa Ra Pa 15 Pc Rc Ra", "Pa:Kleftshift Ra Pa:L2 Pc:Kx Rc Ra"}, // tap then hold releases the one-shot
	}
	handler := func() EventHandler {
		return newChainHandler(NewTapHoldHandler(int64(0)), NewDefaultHandler(), NewOneShotHandler())
	}
	testHandler(t, handler, configStr, tests)
}

func TestOneShotCombo(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    f+d: one-shot leftshift
`
	tests := [][]string{
		{"Pf Pd Rf Rd Pc Rc", "Pf:Kleftshift Pd:N Rd Pc:Kc Rf Rc"},
	}
	handler := func() EventHandler {
		return newChainHandler(NewComboHandler(int64(20)), NewDefaultHandler(), NewOneShotHandler())
	}
	testHandler(t, handler, configStr, tests)
}
