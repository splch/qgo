package pass

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestParallelizeOpsIndependent(t *testing.T) {
	// H(0), H(1), CNOT(0,1): H(0) and H(1) are independent and in front layer.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("parallel", 2, 0, ops, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(resultOps))
	}

	// The H gates should come before CNOT in the result.
	// Both H(0) and H(1) should be in layer 0, CNOT in layer 1.
	cnotIdx := -1
	for i, op := range resultOps {
		if op.Gate == gate.CNOT {
			cnotIdx = i
			break
		}
	}
	if cnotIdx < 2 {
		t.Errorf("CNOT should be at index 2 (after both H gates), got %d", cnotIdx)
	}

	// Verify circuit depth is 2 (H layer + CNOT layer).
	stats := result.Stats()
	if stats.Depth != 2 {
		t.Errorf("expected depth 2, got %d", stats.Depth)
	}
}

func TestParallelizeOpsAlreadyOptimal(t *testing.T) {
	// H(0), CNOT(0,1): already optimal, no reordering needed.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("optimal", 2, 0, ops, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(resultOps))
	}

	if resultOps[0].Gate != gate.H {
		t.Errorf("op 0: expected H, got %s", resultOps[0].Gate.Name())
	}
	if resultOps[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", resultOps[1].Gate.Name())
	}
}

func TestParallelizeOpsReorder(t *testing.T) {
	// H(0), CNOT(0,1), H(2): H(2) is independent and could be in the first layer.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.H, Qubits: []int{2}},
	}
	c := ir.New("reorder", 3, 0, ops, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(resultOps))
	}

	// H(2) should be moved to layer 0 alongside H(0), before CNOT.
	// Check that both H gates come before the CNOT.
	cnotIdx := -1
	for i, op := range resultOps {
		if op.Gate == gate.CNOT {
			cnotIdx = i
			break
		}
	}
	if cnotIdx != 2 {
		t.Errorf("CNOT should be at index 2, got %d", cnotIdx)
	}
}

func TestParallelizeOpsSingleOp(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("single", 1, 0, ops, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	// For single or no ops, the pass should return the circuit as-is.
	if len(result.Ops()) != 1 {
		t.Errorf("expected 1 op, got %d", len(result.Ops()))
	}
}

func TestParallelizeOpsEmpty(t *testing.T) {
	c := ir.New("empty", 2, 0, nil, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops, got %d", len(result.Ops()))
	}
}

func TestParallelizeOpsPreservesGateCount(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
		{Gate: gate.Y, Qubits: []int{2}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{1, 2}},
	}
	c := ir.New("preserve", 3, 0, ops, nil)

	result, err := ParallelizeOps(c, target.Simulator)
	if err != nil {
		t.Fatalf("ParallelizeOps: %v", err)
	}

	if len(result.Ops()) != 5 {
		t.Errorf("expected 5 ops (preserved), got %d", len(result.Ops()))
	}
}
