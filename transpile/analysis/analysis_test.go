package analysis

import (
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// buildTestCircuit creates: H(0), CNOT(0,1), H(2), CNOT(1,2)
func buildTestCircuit() *ir.Circuit {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.H, Qubits: []int{2}},
		{Gate: gate.CNOT, Qubits: []int{1, 2}},
	}
	return ir.New("test", 3, 0, ops, nil)
}

func TestBuildTimelines(t *testing.T) {
	c := buildTestCircuit()
	tl := BuildTimelines(c)

	if len(tl) != 3 {
		t.Fatalf("expected 3 timelines, got %d", len(tl))
	}

	// Qubit 0: ops 0 (H) and 1 (CNOT)
	if len(tl[0].Ops) != 2 {
		t.Errorf("qubit 0: expected 2 ops, got %d", len(tl[0].Ops))
	} else {
		if tl[0].Ops[0] != 0 || tl[0].Ops[1] != 1 {
			t.Errorf("qubit 0: expected ops [0,1], got %v", tl[0].Ops)
		}
	}

	// Qubit 1: ops 1 (CNOT(0,1)) and 3 (CNOT(1,2))
	if len(tl[1].Ops) != 2 {
		t.Errorf("qubit 1: expected 2 ops, got %d", len(tl[1].Ops))
	} else {
		if tl[1].Ops[0] != 1 || tl[1].Ops[1] != 3 {
			t.Errorf("qubit 1: expected ops [1,3], got %v", tl[1].Ops)
		}
	}

	// Qubit 2: ops 2 (H) and 3 (CNOT(1,2))
	if len(tl[2].Ops) != 2 {
		t.Errorf("qubit 2: expected 2 ops, got %d", len(tl[2].Ops))
	} else {
		if tl[2].Ops[0] != 2 || tl[2].Ops[1] != 3 {
			t.Errorf("qubit 2: expected ops [2,3], got %v", tl[2].Ops)
		}
	}

	// Verify Qubit field is set correctly.
	for i, timeline := range tl {
		if timeline.Qubit != i {
			t.Errorf("timeline[%d].Qubit = %d, want %d", i, timeline.Qubit, i)
		}
	}
}

func TestNextOnQubit(t *testing.T) {
	c := buildTestCircuit()
	tl := BuildTimelines(c)

	// Next on qubit 0 after op 0 (H) should be op 1 (CNOT).
	if got := NextOnQubit(tl, 0, 0); got != 1 {
		t.Errorf("NextOnQubit(0, afterIdx=0) = %d, want 1", got)
	}

	// Next on qubit 0 after op 1 should be -1 (no more).
	if got := NextOnQubit(tl, 0, 1); got != -1 {
		t.Errorf("NextOnQubit(0, afterIdx=1) = %d, want -1", got)
	}

	// Next on qubit 1 after op 1 should be op 3.
	if got := NextOnQubit(tl, 1, 1); got != 3 {
		t.Errorf("NextOnQubit(1, afterIdx=1) = %d, want 3", got)
	}

	// Next on qubit 2 after op 2 should be op 3.
	if got := NextOnQubit(tl, 2, 2); got != 3 {
		t.Errorf("NextOnQubit(2, afterIdx=2) = %d, want 3", got)
	}

	// Out-of-range qubit returns -1.
	if got := NextOnQubit(tl, 5, 0); got != -1 {
		t.Errorf("NextOnQubit(5, afterIdx=0) = %d, want -1", got)
	}
	if got := NextOnQubit(tl, -1, 0); got != -1 {
		t.Errorf("NextOnQubit(-1, afterIdx=0) = %d, want -1", got)
	}
}

func TestPrevOnQubit(t *testing.T) {
	c := buildTestCircuit()
	tl := BuildTimelines(c)

	// Prev on qubit 0 before op 1 (CNOT) should be op 0 (H).
	if got := PrevOnQubit(tl, 0, 1); got != 0 {
		t.Errorf("PrevOnQubit(0, beforeIdx=1) = %d, want 0", got)
	}

	// Prev on qubit 0 before op 0 should be -1 (none before).
	if got := PrevOnQubit(tl, 0, 0); got != -1 {
		t.Errorf("PrevOnQubit(0, beforeIdx=0) = %d, want -1", got)
	}

	// Prev on qubit 1 before op 3 should be op 1.
	if got := PrevOnQubit(tl, 1, 3); got != 1 {
		t.Errorf("PrevOnQubit(1, beforeIdx=3) = %d, want 1", got)
	}

	// Prev on qubit 2 before op 3 should be op 2.
	if got := PrevOnQubit(tl, 2, 3); got != 2 {
		t.Errorf("PrevOnQubit(2, beforeIdx=3) = %d, want 2", got)
	}

	// Out-of-range qubit returns -1.
	if got := PrevOnQubit(tl, 10, 3); got != -1 {
		t.Errorf("PrevOnQubit(10, beforeIdx=3) = %d, want -1", got)
	}
}

func TestBuildTimelinesEmptyCircuit(t *testing.T) {
	c := ir.New("empty", 2, 0, nil, nil)
	tl := BuildTimelines(c)
	if len(tl) != 2 {
		t.Fatalf("expected 2 timelines, got %d", len(tl))
	}
	for i, timeline := range tl {
		if len(timeline.Ops) != 0 {
			t.Errorf("qubit %d: expected 0 ops, got %d", i, len(timeline.Ops))
		}
	}
}
