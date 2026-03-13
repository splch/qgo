package pipeline

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile"
	"github.com/splch/goqu/transpile/target"
)

func TestDefaultPipelineLevelNone(t *testing.T) {
	// Bell circuit: H(0), CNOT(0,1) with Simulator target.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("bell", 2, 0, ops, nil)

	pipeline := DefaultPipeline(LevelNone)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline(LevelNone): %v", err)
	}

	// Simulator accepts all gates, so circuit should pass through largely unchanged.
	if len(result.Ops()) < 2 {
		t.Errorf("expected at least 2 ops, got %d", len(result.Ops()))
	}
}

func TestDefaultPipelineLevelBasicCancellation(t *testing.T) {
	// Circuit with cancellable gates: H(0), H(0), CNOT(0,1).
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cancel", 2, 0, ops, nil)

	pipeline := DefaultPipeline(LevelBasic)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline(LevelBasic): %v", err)
	}

	// The two H gates should cancel, leaving just CNOT.
	if len(result.Ops()) > 2 {
		t.Errorf("expected at most 2 ops after cancellation, got %d", len(result.Ops()))
	}
}

func TestDefaultPipelineLevelBasicMerge(t *testing.T) {
	// Circuit with mergeable rotations: RZ(0.3)(0), RZ(0.4)(0).
	ops := []ir.Operation{
		{Gate: gate.RZ(0.3), Qubits: []int{0}},
		{Gate: gate.RZ(0.4), Qubits: []int{0}},
	}
	c := ir.New("merge", 1, 0, ops, nil)

	pipeline := DefaultPipeline(LevelBasic)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline(LevelBasic): %v", err)
	}

	// The two RZ gates should merge into one.
	if len(result.Ops()) > 1 {
		t.Errorf("expected at most 1 op after merge, got %d", len(result.Ops()))
	}
}

func TestDefaultCost(t *testing.T) {
	// Circuit: H(0), CNOT(0,1)
	// Stats: Depth=2, GateCount=2, TwoQubitGates=1
	// Cost = 10*1 + 2 + 0.1*2 = 12.2
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cost", 2, 0, ops, nil)

	cost := DefaultCost(c)

	// Expected: 10*1 + 2 + 0.1*2 = 12.2
	expected := 12.2
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("DefaultCost = %f, want %f", cost, expected)
	}
}

func TestDefaultCostEmpty(t *testing.T) {
	c := ir.New("empty", 2, 0, nil, nil)
	cost := DefaultCost(c)
	if cost != 0.0 {
		t.Errorf("DefaultCost for empty circuit = %f, want 0.0", cost)
	}
}

func TestDefaultPipelineLevelFull(t *testing.T) {
	// Circuit with gates that benefit from commutation + cancellation.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("full", 2, 0, ops, nil)

	pipeline := DefaultPipeline(LevelFull)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline(LevelFull): %v", err)
	}

	// Should still produce a valid result.
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.NumQubits() != 2 {
		t.Errorf("expected 2 qubits, got %d", result.NumQubits())
	}
}

func TestDefaultPipelineLevelParallel(t *testing.T) {
	// Test LevelParallel pipeline with a simple circuit.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("parallel", 2, 0, ops, nil)

	pipeline := DefaultPipeline(LevelParallel)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline(LevelParallel): %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestDefaultPipelineWithBarriers(t *testing.T) {
	// Barriers should be removed by the pipeline.
	c, err := buildBarrierCircuit()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	pipeline := DefaultPipeline(LevelNone)
	result, err := pipeline(c, target.Simulator)
	if err != nil {
		t.Fatalf("DefaultPipeline: %v", err)
	}

	for i, op := range result.Ops() {
		if op.Gate != nil && op.Gate.Name() == "barrier" {
			t.Errorf("op %d is a barrier; should have been removed by pipeline", i)
		}
	}
}

// buildBarrierCircuit uses ir.New to create a circuit with a barrier.
func buildBarrierCircuit() (*ir.Circuit, error) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: testBarrier(2), Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	return ir.New("barrier_test", 2, 0, ops, nil), nil
}

// testBarrier is a minimal barrier gate for testing.
type testBarrier int

func (g testBarrier) Name() string                     { return "barrier" }
func (g testBarrier) Qubits() int                      { return int(g) }
func (g testBarrier) Matrix() []complex128             { return nil }
func (g testBarrier) Params() []float64                { return nil }
func (g testBarrier) Inverse() gate.Gate               { return g }
func (g testBarrier) Decompose(_ []int) []gate.Applied { return nil }

func TestDefaultCostWithTwoQubitGates(t *testing.T) {
	// 3 gates: H, CNOT, CNOT. Stats: Depth=3, GateCount=3, TwoQubitGates=2
	// Cost = 10*2 + 3 + 0.1*3 = 23.3
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("cost2", 2, 0, ops, nil)

	cost := DefaultCost(c)

	expected := 23.3
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("DefaultCost = %f, want %f", cost, expected)
	}
}

func TestOptimizeParallel(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("opt", 2, 0, ops, nil)

	// Use two strategies.
	strategies := []transpile.Pass{
		DefaultPipeline(LevelBasic),
		DefaultPipeline(LevelFull),
	}

	result, err := OptimizeParallel(c, target.Simulator, strategies, DefaultCost)
	if err != nil {
		t.Fatalf("OptimizeParallel: %v", err)
	}
	if result == nil {
		t.Fatal("OptimizeParallel returned nil")
	}
}
