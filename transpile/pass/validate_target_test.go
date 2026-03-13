package pass

import (
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestValidateTargetIBMBasisPass(t *testing.T) {
	// Circuit with only IBM Eagle basis gates: CX, RZ, SX, X.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.RZ(0.5), Qubits: []int{0}},
		{Gate: gate.SX, Qubits: []int{1}},
		{Gate: gate.X, Qubits: []int{0}},
	}
	c := ir.New("ibm_valid", 2, 0, ops, nil)

	// Use a target with IBM basis and all-to-all connectivity (no connectivity issues).
	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	result, err := ValidateTarget(c, tgt)
	if err != nil {
		t.Fatalf("ValidateTarget returned unexpected error: %v", err)
	}
	if result != c {
		t.Error("ValidateTarget should return the same circuit on success")
	}
}

func TestValidateTargetHFailsIBM(t *testing.T) {
	// H is not in the IBM basis.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("h_test", 2, 0, ops, nil)

	tgt := target.Target{
		Name:       "IBM-test",
		NumQubits:  2,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
	}

	_, err := ValidateTarget(c, tgt)
	if err == nil {
		t.Fatal("ValidateTarget should fail for H gate on IBM target")
	}
	if !strings.Contains(err.Error(), "not in target") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateTargetConnectivityViolation(t *testing.T) {
	// CNOT on non-connected qubits.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("conn_test", 3, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-3",
		NumQubits:  3,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
		},
	}

	_, err := ValidateTarget(c, tgt)
	if err == nil {
		t.Fatal("ValidateTarget should fail for non-connected qubits")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateTargetConnectedPairPasses(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("conn_pass", 3, 0, ops, nil)

	tgt := target.Target{
		Name:       "linear-3",
		NumQubits:  3,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
		Connectivity: []target.QubitPair{
			{Q0: 0, Q1: 1},
			{Q0: 1, Q1: 2},
		},
	}

	_, err := ValidateTarget(c, tgt)
	if err != nil {
		t.Fatalf("ValidateTarget should pass for connected qubits: %v", err)
	}
}

func TestValidateTargetSimulatorAcceptsAll(t *testing.T) {
	// Simulator target accepts all gates ("*" in basis).
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.RZ(1.23), Qubits: []int{0}},
		{Gate: gate.CCX, Qubits: []int{0, 1, 2}},
	}
	c := ir.New("sim_test", 3, 0, ops, nil)

	_, err := ValidateTarget(c, target.Simulator)
	if err != nil {
		t.Fatalf("ValidateTarget should pass for Simulator: %v", err)
	}
}

func TestValidateTargetDepthLimit(t *testing.T) {
	// Create a circuit that exceeds depth limit.
	ops := []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
	}
	c := ir.New("depth_test", 1, 0, ops, nil)

	tgt := target.Target{
		Name:            "depth-limited",
		NumQubits:       1,
		BasisGates:      []string{"X"},
		MaxCircuitDepth: 2,
	}

	_, err := ValidateTarget(c, tgt)
	if err == nil {
		t.Fatal("ValidateTarget should fail for circuit exceeding depth limit")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateTargetQubitCountExceeded(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
	}
	c := ir.New("qubit_test", 5, 0, ops, nil)

	tgt := target.Target{
		Name:       "small",
		NumQubits:  3,
		BasisGates: []string{"X"},
	}

	_, err := ValidateTarget(c, tgt)
	if err == nil {
		t.Fatal("ValidateTarget should fail when circuit qubits exceed target")
	}
	if !strings.Contains(err.Error(), "qubits") {
		t.Errorf("unexpected error message: %v", err)
	}
}
