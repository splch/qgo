package pass

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestCancelAdjacentHH(t *testing.T) {
	// H(0), H(0) -> should cancel entirely.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("hh", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after H-H cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentSSdg(t *testing.T) {
	// S(0), Sdg(0) -> should cancel.
	ops := []ir.Operation{
		{Gate: gate.S, Qubits: []int{0}},
		{Gate: gate.Sdg, Qubits: []int{0}},
	}
	c := ir.New("s_sdg", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after S-Sdg cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentRZInverse(t *testing.T) {
	// RZ(0.5)(0), RZ(-0.5)(0) -> should cancel (params sum to 0).
	ops := []ir.Operation{
		{Gate: gate.RZ(0.5), Qubits: []int{0}},
		{Gate: gate.RZ(-0.5), Qubits: []int{0}},
	}
	c := ir.New("rz_inv", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after RZ/RZ-inverse cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentCNOT(t *testing.T) {
	// CNOT(0,1), CNOT(0,1) -> should cancel.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cnot_cnot", 2, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after CNOT-CNOT cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentXX(t *testing.T) {
	// X(0), X(0) -> should cancel (X is self-inverse).
	ops := []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
	}
	c := ir.New("xx", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after X-X cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentNonAdjacent(t *testing.T) {
	// H(0), X(0), H(0) -> H and H are not adjacent, so no cancellation.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("non_adj", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 3 {
		t.Errorf("expected 3 ops (no cancellation), got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentDifferentQubits(t *testing.T) {
	// H(0), H(1) -> different qubits, no cancellation.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{1}},
	}
	c := ir.New("diff_q", 2, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops (no cancellation), got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentMultiplePairs(t *testing.T) {
	// H(0), H(0), X(1), X(1) -> both pairs cancel.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	c := ir.New("multi", 2, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after all cancellations, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentTTdg(t *testing.T) {
	// T(0), Tdg(0) -> should cancel.
	ops := []ir.Operation{
		{Gate: gate.T, Qubits: []int{0}},
		{Gate: gate.Tdg, Qubits: []int{0}},
	}
	c := ir.New("t_tdg", 1, 0, ops, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after T-Tdg cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentEmptyCircuit(t *testing.T) {
	c := ir.New("empty", 1, 0, nil, nil)

	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops for empty circuit, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentCascading(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("hhhh", 1, 0, ops, nil)
	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}
	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after H H H H cancellation, got %d", len(result.Ops()))
	}
}

func TestCancelAdjacentTriple(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("hhh", 1, 0, ops, nil)
	result, err := CancelAdjacent(c, target.Simulator)
	if err != nil {
		t.Fatalf("CancelAdjacent: %v", err)
	}
	if len(result.Ops()) != 1 {
		t.Errorf("expected 1 op after H H H cancellation, got %d", len(result.Ops()))
	}
}
