package verify

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestEquivalentOnZeroIdentical(t *testing.T) {
	// Two identical circuits should be equivalent.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	a := ir.New("a", 2, 0, ops, nil)
	b := ir.New("b", 2, 0, ops, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("identical circuits should be equivalent")
	}
}

func TestEquivalentOnZeroDifferent(t *testing.T) {
	// H(0) vs X(0) should not be equivalent.
	opsA := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}
	opsB := []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
	}
	a := ir.New("a", 1, 0, opsA, nil)
	b := ir.New("b", 1, 0, opsB, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if eq {
		t.Error("H(0) and X(0) should not be equivalent on |0>")
	}
}

func TestEquivalentOnZeroGlobalPhase(t *testing.T) {
	// RZ(theta) and RZ(theta) should be equivalent (up to global phase).
	opsA := []ir.Operation{
		{Gate: gate.RZ(1.0), Qubits: []int{0}},
	}
	opsB := []ir.Operation{
		{Gate: gate.RZ(1.0), Qubits: []int{0}},
	}
	a := ir.New("a", 1, 0, opsA, nil)
	b := ir.New("b", 1, 0, opsB, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("same RZ circuits should be equivalent")
	}
}

func TestEquivalentOnZeroEmptyCircuits(t *testing.T) {
	a := ir.New("a", 2, 0, nil, nil)
	b := ir.New("b", 2, 0, nil, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("two empty circuits should be equivalent")
	}
}

func TestEquivalentOnZeroQubitMismatch(t *testing.T) {
	a := ir.New("a", 2, 0, nil, nil)
	b := ir.New("b", 3, 0, nil, nil)

	_, err := EquivalentOnZero(a, b, 1e-8)
	if err == nil {
		t.Fatal("expected error for qubit count mismatch")
	}
}

func TestEquivalentOnZeroDecomposedBell(t *testing.T) {
	// Original: H(0), CNOT(0,1) (Bell state)
	original := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	a := ir.New("original", 2, 0, original, nil)

	// Decomposed form using RZ, SX, CX (IBM basis).
	// H = RZ(pi/2) SX RZ(pi/2)
	// CNOT stays as CNOT.
	decomposed := []ir.Operation{
		{Gate: gate.RZ(1.5707963267948966), Qubits: []int{0}},
		{Gate: gate.SX, Qubits: []int{0}},
		{Gate: gate.RZ(1.5707963267948966), Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	b := ir.New("decomposed", 2, 0, decomposed, nil)

	eq, err := EquivalentOnZero(a, b, 1e-6)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("original and decomposed Bell circuits should be equivalent")
	}
}

func TestEquivalentOnZeroIdentityVsEmpty(t *testing.T) {
	// Identity gate should be equivalent to empty circuit.
	a := ir.New("identity", 1, 0, []ir.Operation{
		{Gate: gate.I, Qubits: []int{0}},
	}, nil)
	b := ir.New("empty", 1, 0, nil, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("identity gate should be equivalent to empty circuit")
	}
}

func TestEquivalentOnZeroSelfInverse(t *testing.T) {
	// X(0), X(0) should be equivalent to empty.
	a := ir.New("xx", 1, 0, []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
	}, nil)
	b := ir.New("empty", 1, 0, nil, nil)

	eq, err := EquivalentOnZero(a, b, 1e-8)
	if err != nil {
		t.Fatalf("EquivalentOnZero: %v", err)
	}
	if !eq {
		t.Error("X-X should be equivalent to identity")
	}
}

func TestEquivalentOnZeroTooLarge(t *testing.T) {
	a := ir.New("big", 15, 0, nil, nil)
	b := ir.New("big", 15, 0, nil, nil)

	_, err := EquivalentOnZero(a, b, 1e-8)
	if err == nil {
		t.Fatal("expected error for circuit too large (>14 qubits)")
	}
}
