package pass

import (
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile"
	"github.com/splch/qgo/transpile/target"
	"github.com/splch/qgo/transpile/verify"
)

func TestDecomposeToTargetIBM(t *testing.T) {
	// Build circuit: H(0), CNOT(0,1)
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("bell", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	result, err := DecomposeToTarget(c, tgt)
	if err != nil {
		t.Fatalf("DecomposeToTarget: %v", err)
	}

	// Check all gates are in the IBM basis.
	ibmBasis := map[string]bool{"CX": true, "ID": true, "RZ": true, "SX": true, "X": true}
	for i, op := range result.Ops() {
		if op.Gate == nil {
			continue
		}
		bname := transpile.BasisName(op.Gate)
		if !ibmBasis[bname] {
			t.Errorf("op %d: gate %q (basis %q) not in IBM basis", i, op.Gate.Name(), bname)
		}
	}

	// Verify equivalence with the original circuit.
	eq, err := verify.EquivalentOnZero(c, result, 1e-8)
	if err != nil {
		t.Fatalf("equivalence check: %v", err)
	}
	if !eq {
		t.Error("decomposed circuit is not equivalent to original")
	}
}

func TestDecomposeToTargetPreservesBasisGates(t *testing.T) {
	// If the circuit is already in basis, it should pass through unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.RZ(0.5), Qubits: []int{0}},
		{Gate: gate.SX, Qubits: []int{1}},
	}
	c := ir.New("basis", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	result, err := DecomposeToTarget(c, tgt)
	if err != nil {
		t.Fatalf("DecomposeToTarget: %v", err)
	}

	if len(result.Ops()) != 3 {
		t.Errorf("expected 3 ops (unchanged), got %d", len(result.Ops()))
	}
}

func TestDecomposeToTargetSimulator(t *testing.T) {
	// Simulator accepts all gates, so nothing should change.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("sim", 2, 0, ops, nil)

	result, err := DecomposeToTarget(c, target.Simulator)
	if err != nil {
		t.Fatalf("DecomposeToTarget: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops for simulator, got %d", len(result.Ops()))
	}
}

func TestDecomposeToTargetSWAP(t *testing.T) {
	// SWAP should decompose to 3 CX gates for IBM basis.
	ops := []ir.Operation{
		{Gate: gate.SWAP, Qubits: []int{0, 1}},
	}
	c := ir.New("swap", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	result, err := DecomposeToTarget(c, tgt)
	if err != nil {
		t.Fatalf("DecomposeToTarget: %v", err)
	}

	// SWAP = 3 CX gates, so expect at least 3 ops.
	cxCount := 0
	for _, op := range result.Ops() {
		if op.Gate != nil && transpile.BasisName(op.Gate) == "CX" {
			cxCount++
		}
	}
	if cxCount < 3 {
		t.Errorf("expected at least 3 CX gates for SWAP decomposition, got %d", cxCount)
	}

	// Verify equivalence.
	eq, err := verify.EquivalentOnZero(c, result, 1e-8)
	if err != nil {
		t.Fatalf("equivalence check: %v", err)
	}
	if !eq {
		t.Error("decomposed SWAP circuit is not equivalent to original")
	}
}

func TestDecomposeToTargetRemovesBarriers(t *testing.T) {
	// DecomposeToTarget should skip barriers (they are stripped).
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: barrierGate(2), Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("barrier", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	result, err := DecomposeToTarget(c, tgt)
	if err != nil {
		t.Fatalf("DecomposeToTarget: %v", err)
	}

	// No barriers should remain.
	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "barrier" {
			t.Errorf("op %d is a barrier; should have been removed", i)
		}
	}
}

// barrierGate mimics the builder's barrier gate for testing.
type barrierGate int

func (g barrierGate) Name() string            { return "barrier" }
func (g barrierGate) Qubits() int             { return int(g) }
func (g barrierGate) Matrix() []complex128     { return nil }
func (g barrierGate) Params() []float64       { return nil }
func (g barrierGate) Inverse() gate.Gate      { return g }
func (g barrierGate) Decompose(_ []int) []gate.Applied { return nil }
