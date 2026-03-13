package emitter

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
)

func TestEmitBell(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if s == "" {
		t.Fatal("empty output")
	}

	expects := []string{
		"DECLARE ro BIT[2]",
		"H 0",
		"CNOT 0 1",
		"MEASURE 0 ro[0]",
		"MEASURE 1 ro[1]",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitGHZ(t *testing.T) {
	c, err := builder.New("ghz", 3).
		H(0).
		CNOT(0, 1).
		CNOT(1, 2).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"DECLARE ro BIT[3]",
		"H 0",
		"CNOT 0 1",
		"CNOT 1 2",
		"MEASURE 0 ro[0]",
		"MEASURE 1 ro[1]",
		"MEASURE 2 ro[2]",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitParameterized(t *testing.T) {
	c, err := builder.New("param", 1).
		WithClbits(1).
		RZ(math.Pi/4, 0).
		Phase(math.Pi/2, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "RZ(") {
		t.Errorf("output missing RZ gate\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "PHASE(") {
		t.Errorf("output missing PHASE gate\nFull output:\n%s", s)
	}
}

func TestEmitFixedGates(t *testing.T) {
	c, err := builder.New("gates", 2).
		WithClbits(2).
		H(0).
		X(0).
		Y(0).
		Z(0).
		S(0).
		Apply(gate.Sdg, 0).
		T(0).
		Apply(gate.Tdg, 0).
		CZ(0, 1).
		SWAP(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"H 0",
		"X 0",
		"Y 0",
		"Z 0",
		"S 0",
		"DAGGER S 0",
		"T 0",
		"DAGGER T 0",
		"CZ 0 1",
		"SWAP 0 1",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitSX(t *testing.T) {
	c, err := builder.New("sx", 1).
		WithClbits(1).
		Apply(gate.SX, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	// SX should be decomposed to RX(pi/2).
	if !strings.Contains(s, "RX(pi/2)") {
		t.Errorf("SX should be decomposed to RX(pi/2)\nFull output:\n%s", s)
	}
}

func TestEmitCCX(t *testing.T) {
	c, err := builder.New("ccx", 3).
		WithClbits(3).
		Apply(gate.CCX, 0, 1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "CCNOT 0 1 2") {
		t.Errorf("output missing CCNOT\nFull output:\n%s", s)
	}
}

func TestEmitControlled(t *testing.T) {
	c, err := builder.New("ctrl", 4).
		WithClbits(4).
		Apply(gate.MCX(3), 0, 1, 2, 3).
		Apply(gate.Controlled(gate.H, 2), 0, 1, 2).
		Apply(gate.MCZ(2), 0, 1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"CONTROLLED CONTROLLED CONTROLLED X 0 1 2 3",
		"CONTROLLED CONTROLLED H 0 1 2",
		"CONTROLLED CONTROLLED Z 0 1 2",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitControlledParameterized(t *testing.T) {
	c, err := builder.New("ctrl-param", 2).
		WithClbits(2).
		Apply(gate.CRZ(math.Pi/4), 0, 1).
		Apply(gate.CP(math.Pi/2), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "CONTROLLED RZ(") {
		t.Errorf("output missing CONTROLLED RZ\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "CONTROLLED PHASE(") {
		t.Errorf("output missing CONTROLLED PHASE\nFull output:\n%s", s)
	}
}

func TestEmitReset(t *testing.T) {
	c, err := builder.New("reset", 1).
		WithClbits(1).
		Reset(0).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "RESET 0") {
		t.Errorf("output missing RESET\nFull output:\n%s", s)
	}
}

func TestEmitWithComments(t *testing.T) {
	c, err := builder.New("test-circuit", 1).
		WithClbits(1).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c, WithComments(true))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "# Circuit: test-circuit") {
		t.Errorf("output missing comment\nFull output:\n%s", s)
	}
}

func TestEmitU3Error(t *testing.T) {
	c, err := builder.New("u3", 1).
		WithClbits(1).
		Apply(gate.U3(math.Pi/4, math.Pi/2, math.Pi), 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_, err = EmitString(c)
	if err == nil {
		t.Fatal("expected error for U3 gate")
	}
	if !strings.Contains(err.Error(), "U3") {
		t.Errorf("error should mention U3: %v", err)
	}
}

func TestEmitBarrier(t *testing.T) {
	c, err := builder.New("barrier", 2).
		WithClbits(2).
		H(0).
		Barrier(0, 1).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "PRAGMA PRESERVE_BLOCK") {
		t.Errorf("output missing PRAGMA for barrier\nFull output:\n%s", s)
	}
}

func TestEmitNoClbits(t *testing.T) {
	c, err := builder.New("noclbits", 1).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(s, "DECLARE") {
		t.Errorf("should not have DECLARE when no clbits\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "H 0") {
		t.Errorf("output missing H gate\nFull output:\n%s", s)
	}
}
