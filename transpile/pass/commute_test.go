package pass

import (
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

func TestCommuteThroughCNOT_HDoesNotCommute(t *testing.T) {
	// H(0), CNOT(0,1), H(0): H does not commute with CNOT on control.
	// Order should be preserved.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("h_cnot_h", 2, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(resultOps))
	}

	// H should not commute, so order H, CNOT, H should be preserved.
	if resultOps[0].Gate != gate.H {
		t.Errorf("op 0: expected H, got %s", resultOps[0].Gate.Name())
	}
	if resultOps[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", resultOps[1].Gate.Name())
	}
	if resultOps[2].Gate != gate.H {
		t.Errorf("op 2: expected H, got %s", resultOps[2].Gate.Name())
	}
}

func TestCommuteThroughCNOT_EmptyCircuit(t *testing.T) {
	c := ir.New("empty", 2, 0, nil, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops for empty circuit, got %d", len(result.Ops()))
	}
}

func TestCommuteThroughCNOT_NoSingleQubitGates(t *testing.T) {
	// Circuit with only CNOTs: nothing should change.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cnots_only", 2, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops, got %d", len(result.Ops()))
	}
}

func TestCommuteThroughCNOT_YDoesNotCommuteOnTarget(t *testing.T) {
	// Y(1), CNOT(0,1): Y does not commute through CNOT target.
	// Order should be preserved.
	ops := []ir.Operation{
		{Gate: gate.Y, Qubits: []int{1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("y_tgt", 2, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(resultOps))
	}
	if resultOps[0].Gate != gate.Y {
		t.Errorf("op 0: expected Y (no commutation), got %s", resultOps[0].Gate.Name())
	}
	if resultOps[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", resultOps[1].Gate.Name())
	}
}

func TestCommuteThroughCNOT_ZOnControlDoesNotCommuteOnTarget(t *testing.T) {
	// Z(1), CNOT(0,1): Z is on target (qubit 1), Z does NOT commute with
	// CNOT target (only X-type commutes through target). Order preserved.
	ops := []ir.Operation{
		{Gate: gate.Z, Qubits: []int{1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("z_tgt", 2, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	resultOps := result.Ops()
	if resultOps[0].Gate != gate.Z {
		t.Errorf("op 0: expected Z (no commutation on target), got %s", resultOps[0].Gate.Name())
	}
}

func TestCommuteThroughCNOT_XOnControlDoesNotCommute(t *testing.T) {
	// X(0), CNOT(0,1): X does not commute through CNOT control (only Z-type does).
	// Order should be preserved.
	ops := []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("x_ctrl", 2, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	resultOps := result.Ops()
	if resultOps[0].Gate != gate.X {
		t.Errorf("op 0: expected X (no commutation on control), got %s", resultOps[0].Gate.Name())
	}
}

func TestCommuteThroughCNOT_DifferentQubitsNoInteraction(t *testing.T) {
	// Z(2), CNOT(0,1): Z is on qubit 2, not involved in CNOT. No commutation.
	ops := []ir.Operation{
		{Gate: gate.Z, Qubits: []int{2}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("diff_q", 3, 0, ops, nil)

	result, err := CommuteThroughCNOT(c, target.Simulator)
	if err != nil {
		t.Fatalf("CommuteThroughCNOT: %v", err)
	}

	resultOps := result.Ops()
	if len(resultOps) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(resultOps))
	}
	// Order should be preserved since Z is not on any CNOT qubit.
	if resultOps[0].Gate != gate.Z {
		t.Errorf("op 0: expected Z, got %s", resultOps[0].Gate.Name())
	}
}

func TestCommutesWithControl(t *testing.T) {
	// Test the commutation rules directly.
	commuters := []gate.Gate{gate.Z, gate.S, gate.Sdg, gate.T, gate.Tdg, gate.RZ(0.5)}
	for _, g := range commuters {
		if !commutesWithControl(g) {
			t.Errorf("%s should commute with CNOT control", g.Name())
		}
	}

	nonCommuters := []gate.Gate{gate.H, gate.X, gate.Y, gate.SX}
	for _, g := range nonCommuters {
		if commutesWithControl(g) {
			t.Errorf("%s should NOT commute with CNOT control", g.Name())
		}
	}
}

func TestCommutesWithTarget(t *testing.T) {
	// X and RX commute with CNOT target.
	if !commutesWithTarget(gate.X) {
		t.Error("X should commute with CNOT target")
	}
	if !commutesWithTarget(gate.RX(0.5)) {
		t.Error("RX should commute with CNOT target")
	}

	// Others should not.
	nonCommuters := []gate.Gate{gate.H, gate.Z, gate.Y, gate.S}
	for _, g := range nonCommuters {
		if commutesWithTarget(g) {
			t.Errorf("%s should NOT commute with CNOT target", g.Name())
		}
	}
}
