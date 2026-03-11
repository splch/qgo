package pass

import (
	"math"
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

// directedTarget has asymmetric connectivity: 0->1 and 1->2 only.
var directedTarget = target.Target{
	Name:       "test-directed",
	NumQubits:  3,
	BasisGates: []string{"CX", "CZ", "RZ", "SX", "X", "H", "I", "SWAP", "CP"},
	Connectivity: []target.QubitPair{
		{Q0: 0, Q1: 1}, // 0->1 only
		{Q0: 1, Q1: 2}, // 1->2 only
	},
}

func TestFixDirectionCXNativeDirection(t *testing.T) {
	// CX(0,1) is in native direction -> unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cx_native", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(rops))
	}
	if rops[0].Gate.Name() != "CNOT" {
		t.Errorf("expected CNOT, got %s", rops[0].Gate.Name())
	}
	if rops[0].Qubits[0] != 0 || rops[0].Qubits[1] != 1 {
		t.Errorf("expected qubits [0,1], got %v", rops[0].Qubits)
	}
}

func TestFixDirectionCXReverse(t *testing.T) {
	// CX(1,0) is reverse -> should become H(1) H(0) CX(0,1) H(0) H(1).
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{1, 0}},
	}
	c := ir.New("cx_reverse", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 5 {
		t.Fatalf("expected 5 ops for CX reversal, got %d", len(rops))
	}

	// H(1), H(0), CX(0,1), H(0), H(1)
	expectGates := []string{"H", "H", "CNOT", "H", "H"}
	expectQubits := [][]int{{1}, {0}, {0, 1}, {0}, {1}}

	for i, op := range rops {
		if op.Gate.Name() != expectGates[i] {
			t.Errorf("op %d: expected gate %s, got %s", i, expectGates[i], op.Gate.Name())
		}
		if !sameQubits(op.Qubits, expectQubits[i]) {
			t.Errorf("op %d: expected qubits %v, got %v", i, expectQubits[i], op.Qubits)
		}
	}
}

func TestFixDirectionCZReverse(t *testing.T) {
	// CZ(1,0) is reverse -> should swap operands to CZ(0,1).
	ops := []ir.Operation{
		{Gate: gate.CZ, Qubits: []int{1, 0}},
	}
	c := ir.New("cz_reverse", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 1 {
		t.Fatalf("expected 1 op for CZ swap, got %d", len(rops))
	}
	if rops[0].Gate.Name() != "CZ" {
		t.Errorf("expected CZ, got %s", rops[0].Gate.Name())
	}
	if rops[0].Qubits[0] != 0 || rops[0].Qubits[1] != 1 {
		t.Errorf("expected qubits [0,1], got %v", rops[0].Qubits)
	}
}

func TestFixDirectionSWAPReverse(t *testing.T) {
	// SWAP(1,0) is reverse -> should swap operands to SWAP(0,1).
	ops := []ir.Operation{
		{Gate: gate.SWAP, Qubits: []int{1, 0}},
	}
	c := ir.New("swap_reverse", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 1 {
		t.Fatalf("expected 1 op for SWAP swap, got %d", len(rops))
	}
	if rops[0].Gate.Name() != "SWAP" {
		t.Errorf("expected SWAP, got %s", rops[0].Gate.Name())
	}
	if rops[0].Qubits[0] != 0 || rops[0].Qubits[1] != 1 {
		t.Errorf("expected qubits [0,1], got %v", rops[0].Qubits)
	}
}

func TestFixDirectionCPReverse(t *testing.T) {
	// CP(pi/4)(1,0) is reverse -> should swap operands.
	phi := math.Pi / 4
	ops := []ir.Operation{
		{Gate: gate.CP(phi), Qubits: []int{1, 0}},
	}
	c := ir.New("cp_reverse", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 1 {
		t.Fatalf("expected 1 op for CP swap, got %d", len(rops))
	}
	if baseName(rops[0].Gate) != "CP" {
		t.Errorf("expected CP, got %s", rops[0].Gate.Name())
	}
	if rops[0].Qubits[0] != 0 || rops[0].Qubits[1] != 1 {
		t.Errorf("expected qubits [0,1], got %v", rops[0].Qubits)
	}
	// Verify parameter is preserved.
	params := rops[0].Gate.Params()
	if len(params) != 1 || math.Abs(params[0]-phi) > 1e-12 {
		t.Errorf("expected param %.4f, got %v", phi, params)
	}
}

func TestFixDirectionNoConnectivity(t *testing.T) {
	// No connectivity between q0 and q2 in either direction -> error.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("no_conn", 3, 0, ops, nil)

	_, err := FixDirection(c, directedTarget)
	if err == nil {
		t.Fatal("expected error for unconnected qubits, got nil")
	}
}

func TestFixDirectionAllToAll(t *testing.T) {
	// All-to-all target -> no-op, circuit returned unchanged.
	allToAll := target.Target{
		Name:       "all-to-all",
		NumQubits:  4,
		BasisGates: []string{"*"},
	}
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{2, 0}},
		{Gate: gate.CZ, Qubits: []int{3, 1}},
	}
	c := ir.New("ata", 4, 0, ops, nil)

	result, err := FixDirection(c, allToAll)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	// Should be the same pointer (early return).
	if result != c {
		t.Error("expected same circuit pointer for all-to-all target")
	}
}

func TestFixDirectionSingleQubitUnchanged(t *testing.T) {
	// Single-qubit gates pass through unchanged.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("mixed", 2, 0, ops, nil)

	result, err := FixDirection(c, directedTarget)
	if err != nil {
		t.Fatalf("FixDirection: %v", err)
	}

	rops := result.Ops()
	if len(rops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(rops))
	}
	if rops[0].Gate.Name() != "H" || rops[1].Gate.Name() != "X" || rops[2].Gate.Name() != "CNOT" {
		t.Errorf("unexpected gate names: %s, %s, %s", rops[0].Gate.Name(), rops[1].Gate.Name(), rops[2].Gate.Name())
	}
}

func TestFixDirectionUnsupportedGate(t *testing.T) {
	// CY is not in the supported reversal set -> error.
	ops := []ir.Operation{
		{Gate: gate.CY, Qubits: []int{1, 0}},
	}
	c := ir.New("unsupported", 2, 0, ops, nil)

	_, err := FixDirection(c, directedTarget)
	if err == nil {
		t.Fatal("expected error for unsupported gate CY, got nil")
	}
}
