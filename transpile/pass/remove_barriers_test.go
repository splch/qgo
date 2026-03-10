package pass

import (
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/transpile/target"
)

func TestRemoveBarriers(t *testing.T) {
	c, err := builder.New("test", 2).
		H(0).
		Barrier(0, 1).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveBarriers(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveBarriers: %v", err)
	}

	ops := result.Ops()
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops after barrier removal, got %d", len(ops))
	}
	if ops[0].Gate != gate.H {
		t.Errorf("op 0: expected H, got %s", ops[0].Gate.Name())
	}
	if ops[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", ops[1].Gate.Name())
	}
}

func TestRemoveBarriersMultiple(t *testing.T) {
	c, err := builder.New("test", 2).
		H(0).
		Barrier(0, 1).
		CNOT(0, 1).
		Barrier(0, 1).
		X(1).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveBarriers(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveBarriers: %v", err)
	}

	ops := result.Ops()
	if len(ops) != 3 {
		t.Fatalf("expected 3 ops after barrier removal, got %d", len(ops))
	}
	if ops[0].Gate != gate.H {
		t.Errorf("op 0: expected H, got %s", ops[0].Gate.Name())
	}
	if ops[1].Gate != gate.CNOT {
		t.Errorf("op 1: expected CNOT, got %s", ops[1].Gate.Name())
	}
	if ops[2].Gate != gate.X {
		t.Errorf("op 2: expected X, got %s", ops[2].Gate.Name())
	}
}

func TestRemoveBarriersNoBarriers(t *testing.T) {
	c, err := builder.New("test", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveBarriers(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveBarriers: %v", err)
	}

	if len(result.Ops()) != 2 {
		t.Errorf("expected 2 ops, got %d", len(result.Ops()))
	}
}

func TestRemoveBarriersPreservesMetadata(t *testing.T) {
	c, err := builder.New("test", 2).
		H(0).
		Barrier(0, 1).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := RemoveBarriers(c, target.Simulator)
	if err != nil {
		t.Fatalf("RemoveBarriers: %v", err)
	}

	if result.Name() != "test" {
		t.Errorf("name: got %q, want %q", result.Name(), "test")
	}
	if result.NumQubits() != 2 {
		t.Errorf("numQubits: got %d, want 2", result.NumQubits())
	}
}
