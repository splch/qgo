package pass

import (
	"math"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestMergeRotationsRZ(t *testing.T) {
	// RZ(0.3)(0), RZ(0.4)(0) -> should merge to RZ(0.7)(0).
	ops := []ir.Operation{
		{Gate: gate.RZ(0.3), Qubits: []int{0}},
		{Gate: gate.RZ(0.4), Qubits: []int{0}},
	}
	c := ir.New("merge_rz", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 1 {
		t.Fatalf("expected 1 op after merge, got %d", len(result.Ops()))
	}

	mergedGate := result.Ops()[0].Gate
	params := mergedGate.Params()
	if params == nil || len(params) != 1 {
		t.Fatalf("expected 1 parameter, got %v", params)
	}
	if math.Abs(params[0]-0.7) > 1e-10 {
		t.Errorf("expected merged angle 0.7, got %f", params[0])
	}
}

func TestMergeRotationsCancel(t *testing.T) {
	// RZ(pi)(0), RZ(pi)(0) -> should merge to RZ(2pi) which is ~0, so removed.
	ops := []ir.Operation{
		{Gate: gate.RZ(math.Pi), Qubits: []int{0}},
		{Gate: gate.RZ(math.Pi), Qubits: []int{0}},
	}
	c := ir.New("merge_cancel", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after RZ(pi)+RZ(pi) cancellation, got %d", len(result.Ops()))
	}
}

func TestMergeRotationsRX(t *testing.T) {
	// RX(0.2)(0), RX(0.3)(0) -> should merge to RX(0.5)(0).
	ops := []ir.Operation{
		{Gate: gate.RX(0.2), Qubits: []int{0}},
		{Gate: gate.RX(0.3), Qubits: []int{0}},
	}
	c := ir.New("merge_rx", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 1 {
		t.Fatalf("expected 1 op after merge, got %d", len(result.Ops()))
	}

	params := result.Ops()[0].Gate.Params()
	if params == nil || len(params) != 1 {
		t.Fatalf("expected 1 parameter, got %v", params)
	}
	if math.Abs(params[0]-0.5) > 1e-10 {
		t.Errorf("expected merged angle 0.5, got %f", params[0])
	}
}

func TestMergeRotationsRY(t *testing.T) {
	// RY(0.1)(0), RY(0.2)(0) -> should merge to RY(0.3)(0).
	ops := []ir.Operation{
		{Gate: gate.RY(0.1), Qubits: []int{0}},
		{Gate: gate.RY(0.2), Qubits: []int{0}},
	}
	c := ir.New("merge_ry", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 1 {
		t.Fatalf("expected 1 op after merge, got %d", len(result.Ops()))
	}

	params := result.Ops()[0].Gate.Params()
	if math.Abs(params[0]-0.3) > 1e-10 {
		t.Errorf("expected merged angle 0.3, got %f", params[0])
	}
}

func TestMergeRotationsDifferentAxes(t *testing.T) {
	// RZ(0.3)(0), RX(0.4)(0) -> different axes, should not merge.
	ops := []ir.Operation{
		{Gate: gate.RZ(0.3), Qubits: []int{0}},
		{Gate: gate.RX(0.4), Qubits: []int{0}},
	}
	c := ir.New("diff_axes", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops (no merge), got %d", len(result.Ops()))
	}
}

func TestMergeRotationsDifferentQubits(t *testing.T) {
	// RZ(0.3)(0), RZ(0.4)(1) -> different qubits, should not merge.
	ops := []ir.Operation{
		{Gate: gate.RZ(0.3), Qubits: []int{0}},
		{Gate: gate.RZ(0.4), Qubits: []int{1}},
	}
	c := ir.New("diff_q", 2, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops (no merge), got %d", len(result.Ops()))
	}
}

func TestMergeRotationsNonRotation(t *testing.T) {
	// H(0), H(0) -> not rotations, should not be merged.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("non_rot", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops (no merge for H gates), got %d", len(result.Ops()))
	}
}

func TestMergeRotationsThreeConsecutive(t *testing.T) {
	// RZ(0.1)(0), RZ(0.2)(0), RZ(0.3)(0) -> should merge iteratively to RZ(0.6)(0).
	ops := []ir.Operation{
		{Gate: gate.RZ(0.1), Qubits: []int{0}},
		{Gate: gate.RZ(0.2), Qubits: []int{0}},
		{Gate: gate.RZ(0.3), Qubits: []int{0}},
	}
	c := ir.New("three_rz", 1, 0, ops, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 1 {
		t.Fatalf("expected 1 op after merging three RZs, got %d", len(result.Ops()))
	}

	params := result.Ops()[0].Gate.Params()
	if math.Abs(params[0]-0.6) > 1e-10 {
		t.Errorf("expected merged angle 0.6, got %f", params[0])
	}
}

func TestMergeRotationsEmpty(t *testing.T) {
	c := ir.New("empty", 1, 0, nil, nil)

	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}

	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops for empty circuit, got %d", len(result.Ops()))
	}
}

func TestMergeRotations_AngleNear2Pi(t *testing.T) {
	eps := 0.001
	ops := []ir.Operation{
		{Gate: gate.RZ(math.Pi - eps), Qubits: []int{0}},
		{Gate: gate.RZ(math.Pi + eps), Qubits: []int{0}},
	}
	c := ir.New("near2pi", 1, 0, ops, nil)
	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}
	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after near-2pi cancellation, got %d", len(result.Ops()))
	}
}

func TestMergeRotations_LargeAngles(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.RZ(3 * math.Pi), Qubits: []int{0}},
		{Gate: gate.RZ(3 * math.Pi), Qubits: []int{0}},
	}
	c := ir.New("large_angles", 1, 0, ops, nil)
	result, err := MergeRotations(c, target.Simulator)
	if err != nil {
		t.Fatalf("MergeRotations: %v", err)
	}
	if len(result.Ops()) != 0 {
		t.Errorf("expected 0 ops after large angle cancellation, got %d", len(result.Ops()))
	}
}
