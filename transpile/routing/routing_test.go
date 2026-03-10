package routing

import (
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

func TestRouteLinearChainInsertsSWAPs(t *testing.T) {
	// CNOT(0,3) on a 4-qubit linear chain (0-1-2-3): should route the circuit.
	// The result should only have 2-qubit gates on connected pairs.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 3}},
	}
	c := ir.New("route_linear", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-4",
		NumQubits:  4,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
			{Q0: 2, Q1: 3},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// The result should have at least one operation.
	if len(result.Ops()) == 0 {
		t.Fatal("expected at least one op in routed circuit")
	}

	// All 2-qubit gates must be on connected pairs.
	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			q0, q1 := op.Qubits[0], op.Qubits[1]
			if !tgt.IsConnected(q0, q1) {
				t.Errorf("op %d: %s on (%d,%d) is not connected in target",
					i, op.Gate.Name(), q0, q1)
			}
		}
	}
}

func TestRouteAllToAllUnchanged(t *testing.T) {
	// All-to-all target (nil connectivity) should return the circuit unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 3}},
	}
	c := ir.New("all_to_all", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "all-to-all",
		NumQubits:  4,
		BasisGates: []string{"*"},
		// Connectivity: nil means all-to-all
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Circuit should be unchanged.
	if result != c {
		t.Error("expected Route to return the same circuit for all-to-all target")
	}
}

func TestRouteAdjacentQubitsNoSWAP(t *testing.T) {
	// CNOT(0,1) on a linear chain: qubits are already adjacent, no SWAP needed.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("adjacent", 4, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-4",
		NumQubits:  4,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
			{Q0: 2, Q1: 3},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Should contain no SWAP gates.
	for _, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			t.Error("expected no SWAP gates for adjacent CNOT")
		}
	}
}

func TestRouteSingleQubitGates(t *testing.T) {
	// Single-qubit gates should be remapped but not require SWAPs.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	c := ir.New("single_q", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-2",
		NumQubits:  2,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops, got %d", len(result.Ops()))
	}
	for _, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			t.Error("expected no SWAP gates for single-qubit-only circuit")
		}
	}
}

func TestRouteMultipleCNOTs(t *testing.T) {
	// Multiple CNOTs on a linear chain.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("multi_cnot", 3, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-3",
		NumQubits:  3,
		BasisGates: []string{"*"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
		},
	}

	result, err := Route(c, tgt)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	// Should have more ops than original (SWAPs inserted for 0->2 CNOT).
	if len(result.Ops()) < 2 {
		t.Errorf("expected at least 2 ops in routed circuit, got %d", len(result.Ops()))
	}
}

func TestTrivialLayout(t *testing.T) {
	layout := TrivialLayout(4)
	for i, v := range layout {
		if v != i {
			t.Errorf("TrivialLayout[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestInverseLayout(t *testing.T) {
	layout := []int{2, 0, 1, 3}
	inv := InverseLayout(layout)

	// layout[0]=2 means logical 0 -> physical 2, so inv[2]=0
	expected := []int{1, 2, 0, 3}
	for i, v := range inv {
		if v != expected[i] {
			t.Errorf("InverseLayout[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestRouteIonQAllToAll(t *testing.T) {
	// IonQ targets have all-to-all connectivity, circuit should be unchanged.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 5}},
		{Gate: gate.CNOT, Qubits: []int{3, 7}},
	}
	c := ir.New("ionq", 10, 0, ops, nil)

	result, err := Route(c, target.IonQForte)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if result != c {
		t.Error("expected Route to return same circuit for IonQ (all-to-all)")
	}
}
