package handlers

import (
	"testing"
)

func TestComboUnmapped(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a+b: x
`
	tests := [][]string{
		{"Pc Rc", "Pc Rc"}, // events not mapped should be passed through
		{"Pd Pc Rd Rc", "Pd Pc Rd Rc"},
		{"Pa:Km 15 Ra", "Pa:Km Ra"}, // event already mapped to a binding
	}
	handler := func() EventHandler { return NewComboHandler(int64(10)) }
	testHandler(t, handler, configStr, tests)
}

func TestCombo(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a+b: x
    c: c
`
	tests := [][]string{
		{"Pa Ra", "Pa Ra"}, // not triggered
		{"Pb Rb", "Pb Rb"},
		{"Pa Ra Pb Rb", "Pa Ra Pb Rb"},
		{"Pa 15 Pb Ra Rb", "Pa Pb Ra Rb"},          // too slow
		{"Pa Pc Pb Ra Rb Rc", "Pa Pc Pb Ra Rb Rc"}, // interrupted by another key
		{"Pc Pa Rc Pb Ra Rb", "Pc Pa Rc Pb Ra Rb"},
		{"Pa Pb Ra Rb", "Pa:Kx Pb:N Ra Rb"}, // triggered
		{"Pb Pa Rb Ra", "Pb:Kx Pa:N Rb Ra"},
		{"Pb Pa Ra Rb", "Pb:Kx Pa:N Ra Rb"},
	}
	handler := func() EventHandler { return NewComboHandler(int64(10)) }
	testHandler(t, handler, configStr, tests)
}

func TestComboSwitchLayer(t *testing.T) {
	configStr := `
layers:
- name: 1
  bindings:
    a+b: toggle-layer 2
    c: c
- name: 2
  bindings:
    d: d
    e+f: y
`
	tests := [][]string{
		{"Pa Pb Ra Rb", "Pa:L2 Pb:N Ra Rb"},
		{"Pa Pb Pd Ra Rb Rd", "Pa:L2 Pb:N Pd Ra Rb Rd"},
		{"Pa Pb Pe Pf Re Rf Ra Rb", "Pa:L2 Pb:N Pe:Ky Pf:N Re Rf Ra Rb"},
		{"Pa Pb 15 Pe Pf Re Rf Ra Rb", "Pa:L2 Pb:N Pe:Ky Pf:N Re Rf Ra Rb"},
	}
	handler := func() EventHandler { return NewComboHandler(int64(10)) }
	testHandler(t, handler, configStr, tests)
}
