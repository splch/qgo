package transpile

import (
	"fmt"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestBasisName(t *testing.T) {
	tests := []struct {
		gate gate.Gate
		want string
	}{
		{gate.CNOT, "CX"},
		{gate.H, "H"},
		{gate.X, "X"},
		{gate.Y, "Y"},
		{gate.Z, "Z"},
		{gate.S, "S"},
		{gate.T, "T"},
		{gate.I, "I"},
		{gate.SX, "SX"},
		{gate.CZ, "CZ"},
		{gate.SWAP, "SWAP"},
		{gate.RZ(0.78), "RZ"},
		{gate.RX(1.23), "RX"},
		{gate.RY(0.5), "RY"},
		{gate.Phase(0.3), "P"},
		{gate.U3(1.0, 2.0, 3.0), "U3"},
		{gate.CP(0.5), "CP"},
		{gate.GPI(0.1), "GPI"},
		{gate.GPI2(0.2), "GPI2"},
		{gate.MS(0.1, 0.2), "MS"},
		// Dagger suffix should be stripped.
		{gate.S.Inverse(), "S"},
		{gate.T.Inverse(), "T"},
		{gate.H.Inverse(), "H"},
		// Parameterized inverse: name has both dagger and params.
		{gate.RZ(0.78).Inverse(), "RZ"},
	}

	for _, tt := range tests {
		t.Run(tt.gate.Name(), func(t *testing.T) {
			got := BasisName(tt.gate)
			if got != tt.want {
				t.Errorf("BasisName(%q) = %q, want %q", tt.gate.Name(), got, tt.want)
			}
		})
	}
}

// appendSuffix is a trivial pass that appends a suffix to the circuit name.
func appendSuffix(suffix string) Pass {
	return func(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
		newName := c.Name() + suffix
		return ir.New(newName, c.NumQubits(), c.NumClbits(), c.Ops(), c.Metadata()), nil
	}
}

// failingPass always returns an error.
func failingPass(msg string) Pass {
	return func(_ *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
		return nil, fmt.Errorf("%s", msg)
	}
}

func TestPipelineEmpty(t *testing.T) {
	circ := ir.New("test", 2, 0, nil, nil)
	tgt := target.Simulator

	composed := Pipeline()
	result, err := composed(circ, tgt)
	if err != nil {
		t.Fatalf("Pipeline() returned error: %v", err)
	}
	if result.Name() != "test" {
		t.Errorf("Pipeline() changed circuit name to %q, want %q", result.Name(), "test")
	}
}

func TestPipelineSinglePass(t *testing.T) {
	circ := ir.New("base", 2, 0, nil, nil)
	tgt := target.Simulator

	composed := Pipeline(appendSuffix("_A"))
	result, err := composed(circ, tgt)
	if err != nil {
		t.Fatalf("Pipeline(A) returned error: %v", err)
	}
	if result.Name() != "base_A" {
		t.Errorf("got name %q, want %q", result.Name(), "base_A")
	}
}

func TestPipelineMultiplePasses(t *testing.T) {
	circ := ir.New("base", 2, 0, nil, nil)
	tgt := target.Simulator

	composed := Pipeline(appendSuffix("_A"), appendSuffix("_B"), appendSuffix("_C"))
	result, err := composed(circ, tgt)
	if err != nil {
		t.Fatalf("Pipeline(A,B,C) returned error: %v", err)
	}
	want := "base_A_B_C"
	if result.Name() != want {
		t.Errorf("got name %q, want %q", result.Name(), want)
	}
}

func TestPipelineErrorStopsExecution(t *testing.T) {
	circ := ir.New("base", 2, 0, nil, nil)
	tgt := target.Simulator

	// The failing pass is second; the third pass should never run.
	composed := Pipeline(
		appendSuffix("_A"),
		failingPass("pass2 failed"),
		appendSuffix("_C"),
	)
	_, err := composed(circ, tgt)
	if err == nil {
		t.Fatal("Pipeline expected error, got nil")
	}
	if err.Error() != "pass2 failed" {
		t.Errorf("got error %q, want %q", err.Error(), "pass2 failed")
	}
}

func TestPipelinePreservesOps(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	circ := ir.New("qc", 2, 0, ops, nil)
	tgt := target.Simulator

	// Identity pipeline should preserve operations.
	composed := Pipeline(appendSuffix(""))
	result, err := composed(circ, tgt)
	if err != nil {
		t.Fatalf("Pipeline returned error: %v", err)
	}
	if len(result.Ops()) != 2 {
		t.Errorf("got %d ops, want 2", len(result.Ops()))
	}
	if result.NumQubits() != 2 {
		t.Errorf("got %d qubits, want 2", result.NumQubits())
	}
}
