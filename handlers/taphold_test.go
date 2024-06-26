package handlers

import (
	"testing"
)

func TestUnmapped(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold a ; x ; 10
`
	tests := [][]string{
		{"Pc Rc", "Pc Rc"}, // events not mapped to tap-hold should be passed through
		{"Pd Pc Rd Rc", "Pd Pc Rd Rc"},
		{"Pa:Km 15 Ra", "Pa:Km Ra"}, // event already mapped to a binding
	}
	handler := func() EventHandler { return NewTapHoldHandler(int64(50)) }
	testHandler(t, handler, configStr, tests)
}

func TestTapHold(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold a ; x ; 10
    b: tap-hold b ; toggle-layer 2 ; 10
    c: c
- name: 2
  bindings:
    a: a
    c: tap-hold c ; m ; 10
    d: tap-hold d ; toggle-layer 3 ; 10
- name: 3
  bindings:
    a: a
`
	tests := [][]string{
		{"Pa Ra", "Pa:Ka Ra"}, // tap
		{"Pa 5 Ra", "Pa:Ka Ra"},
		{"Pa 15 Ra", "Pa:Kx Ra"}, // hold
		{"Pa 15 Ra Pa Ra", "Pa:Kx Ra Pa:Ka Ra"},
		{"Pa Pc Ra Rc", "Pa:Ka Pc Ra Rc"}, // tap
		{"Pa Pc Rc Ra", "Pa:Ka Pc Rc Ra"},
		{"Pa Pc Rc 15 Ra", "Pa:Kx Pc Rc Ra"},
		{"Pc Pa Rc 15 Ra", "Pc Rc Pa:Kx Ra"}, // the event order is changed (to prevent c from being held until the tap-hold decision)
		{"Pc Pa Rc Ra", "Pc Rc Pa:Ka Ra"},    // order changed as well
		{"Pb 5 Rb", "Pb:Kb Rb"},              // tap-hold in combination with toggle-layer
		{"Pb 15 Rb", "Pb:L2 Rb"},
		{"Pb 15 Pa Ra Rb", "Pb:L2 Pa Ra Rb"}, // a is just passed through
		{"Pb 15 Pa Rb Ra", "Pb:L2 Pa Rb Ra"},
		{"Pb 15 Pc Rc Rb", "Pb:L2 Pc:Kc Rc Rb"},
		{"Pb 7 Pc 7 Rc Rb", "Pb:L2 Pc:Kc Rc Rb"},
		{"Pb 15 Pc 15 Rc Rb", "Pb:L2 Pc:Km Rc Rb"}, // two holds triggered
		{"Pb 15 Pc 15 Rb Rc", "Pb:L2 Pc:Km Rb Rc"},
		{"Pb Pc 15 Rc Rb", "Pb:L2 Pc:Km Rc Rb"},
		{"Pb Pd 15 Rd Rb", "Pb:L2 Pd:L3 Rd Rb"}, // two toggle-layer
	}
	handler := func() EventHandler { return NewTapHoldHandler(int64(50)) }
	testHandler(t, handler, configStr, tests)
}

func TestTapHoldNext(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold-next a ; x ; 10
    b: tap-hold b ; y ; 10
    c: c
`
	tests := [][]string{
		{"Pa Ra", "Pa:Ka Ra"}, // tap
		{"Pa 5 Ra", "Pa:Ka Ra"},
		{"Pa 15 Ra", "Pa:Kx Ra"}, // hold
		{"Pa 15 Ra Pa Ra", "Pa:Kx Ra Pa:Ka Ra"},
		{"Pa Pc Ra Rc", "Pa:Kx Pc Ra Rc"}, // hold since c is pressed before release of a
		{"Pa Pc Rc Ra", "Pa:Kx Pc Rc Ra"},
		{"Pa Pc Rc 15 Ra", "Pa:Kx Pc Rc Ra"},
		{"Pc Pa Rc 15 Ra", "Pc Rc Pa:Kx Ra"}, // order changed as with tap-hold
		{"Pc Pa Rc Ra", "Pc Rc Pa:Ka Ra"},    // tap
		{"Pa Pb Ra Rb", "Pa:Kx Ra Pb:Kb Rb"}, // in combination with tap-hold
		{"Pa Pb Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb Pa Rb Ra", "Pb:Kb Rb Pa:Ka Ra"},
		{"Pb Pa Ra Rb", "Pb:Kb Pa:Ka Ra Rb"},
		{"Pa 15 Pb Ra Rb", "Pa:Kx Ra Pb:Kb Rb"}, // pauses at different positions
		{"Pa 15 Pb Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb 15 Pa Rb Ra", "Pb:Ky Rb Pa:Ka Ra"},
		{"Pb 15 Pa Ra Rb", "Pb:Ky Pa:Ka Ra Rb"},
		{"Pa Pb 15 Ra Rb", "Pa:Kx Pb:Ky Ra Rb"},
		{"Pa Pb 15 Rb Ra", "Pa:Kx Pb:Ky Rb Ra"},
		{"Pb Pa 15 Rb Ra", "Pb:Ky Pa:Kx Rb Ra"},
		{"Pb Pa 15 Ra Rb", "Pb:Ky Pa:Kx Ra Rb"},
		{"Pa 7 Pb 7 Ra Rb", "Pa:Kx Ra Pb:Kb Rb"},
		{"Pa 7 Pb 7 Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb 7 Pa 7 Rb Ra", "Pb:Ky Rb Pa:Ka Ra"},
		{"Pb 7 Pa 7 Ra Rb", "Pb:Ky Pa:Ka Ra Rb"},
	}
	handler := func() EventHandler { return NewTapHoldHandler(int64(50)) }
	testHandler(t, handler, configStr, tests)
}

func TestTapHoldNextRelease(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold-next-release a ; x ; 10
    b: tap-hold b ; y ; 10
    c: c
`
	tests := [][]string{
		{"Pa Ra", "Pa:Ka Ra"}, // tap
		{"Pa 5 Ra", "Pa:Ka Ra"},
		{"Pa 15 Ra", "Pa:Kx Ra"}, // hold
		{"Pa 15 Ra Pa Ra", "Pa:Kx Ra Pa:Ka Ra"},
		{"Pa Pc Ra Rc", "Pa:Ka Pc Ra Rc"}, // hold since c is pressed before release of a
		{"Pa Pc Rc Ra", "Pa:Kx Pc Rc Ra"},
		{"Pa Pc Rc 15 Ra", "Pa:Kx Pc Rc Ra"},
		{"Pc Pa Rc 15 Ra", "Pc Rc Pa:Kx Ra"}, // order changed as with tap-hold
		{"Pc Pa Rc Ra", "Pc Rc Pa:Ka Ra"},    // tap since c was pressed before a
		{"Pa Pb Ra Rb", "Pa:Ka Ra Pb:Kb Rb"}, // in combination with tap-hold
		{"Pa Pb Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb Pa Rb Ra", "Pb:Kb Rb Pa:Ka Ra"},
		{"Pb Pa Ra Rb", "Pb:Kb Pa:Ka Ra Rb"},
		{"Pa 15 Pb Ra Rb", "Pa:Kx Ra Pb:Kb Rb"}, // pauses at different positions
		{"Pa 15 Pb Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb 15 Pa Rb Ra", "Pb:Ky Rb Pa:Ka Ra"},
		{"Pb 15 Pa Ra Rb", "Pb:Ky Pa:Ka Ra Rb"},
		{"Pa Pb 15 Ra Rb", "Pa:Kx Pb:Ky Ra Rb"},
		{"Pa Pb 15 Rb Ra", "Pa:Kx Pb:Ky Rb Ra"},
		{"Pb Pa 15 Rb Ra", "Pb:Ky Pa:Kx Rb Ra"},
		{"Pb Pa 15 Ra Rb", "Pb:Ky Pa:Kx Ra Rb"},
		{"Pa 7 Pb 7 Ra Rb", "Pa:Kx Ra Pb:Kb Rb"},
		{"Pa 7 Pb 7 Rb Ra", "Pa:Kx Pb:Kb Rb Ra"},
		{"Pb 7 Pa 7 Rb Ra", "Pb:Ky Rb Pa:Ka Ra"},
		{"Pb 7 Pa 7 Ra Rb", "Pb:Ky Pa:Ka Ra Rb"},
	}
	handler := func() EventHandler { return NewTapHoldHandler(int64(50)) }
	testHandler(t, handler, configStr, tests)
}

func TestQuickTap(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a: tap-hold a ; x ; 20
    b: tap-hold-next b ; y ; 20
    c: tap-hold-next-release c ; z ; 20
`
	tests := [][]string{
		{"Pa 5 Ra Pa 30 Ra", "Pa:Ka Ra Pa:Ka Ra"},
		{"Pa 15 Ra Pa 30 Ra", "Pa:Ka Ra Pa:Kx Ra"},
		{"Pb 5 Rb Pb 30 Rb", "Pb:Kb Rb Pb:Kb Rb"},
		{"Pb 15 Rb Pb 30 Rb", "Pb:Kb Rb Pb:Ky Rb"},
		{"Pc 5 Rc Pc 30 Rc", "Pc:Kc Rc Pc:Kc Rc"},
		{"Pc 15 Rc Pc 30 Rc", "Pc:Kc Rc Pc:Kz Rc"},
		{"Pa 5 Ra Pa 30 Ra Pa 30 Ra", "Pa:Ka Ra Pa:Ka Ra Pa:Kx Ra"},
		{"Pa 5 Pd Ra Rd Pa 30 Ra", "Pa:Ka Pd Ra Rd Pa:Ka Ra"},
	}
	var quickTapTime int64 = 10
	handler := func() EventHandler { return NewTapHoldHandler(int64(quickTapTime)) }
	testHandler(t, handler, configStr, tests)
}
