package densitymatrix

import (
	"math"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
)

// TestDMDynamicSimpleConditional: X(0), measure(0→c0), if(c0==1, X, 1).
func TestDMDynamicSimpleConditional(t *testing.T) {
	c, err := builder.New("cond", 2).WithClbits(1).
		X(0).
		Measure(0, 0).
		If(0, 1, gate.X, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if counts["1"] != 1000 {
		t.Errorf("expected all '1', got %v", counts)
	}
}

// TestDMDynamicConditionalNotFired: measure(0→c0) with q0=|0>, if(c0==1, X, 1).
func TestDMDynamicConditionalNotFired(t *testing.T) {
	c, err := builder.New("cond_no", 2).WithClbits(1).
		Measure(0, 0).
		If(0, 1, gate.X, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if counts["0"] != 1000 {
		t.Errorf("expected all '0', got %v", counts)
	}
}

// TestDMDynamicResetAndReuse: H(0), measure(0→c0), reset(0), H(0), measure(0→c1).
func TestDMDynamicResetAndReuse(t *testing.T) {
	c, err := builder.New("reset_reuse", 1).WithClbits(2).
		H(0).
		Measure(0, 0).
		Reset(0).
		H(0).
		Measure(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	c1zero := 0
	c1one := 0
	for bs, n := range counts {
		if len(bs) == 2 && bs[0] == '1' {
			c1one += n
		} else {
			c1zero += n
		}
	}

	ratio := float64(c1one) / float64(c1zero+c1one)
	if math.Abs(ratio-0.5) > 0.05 {
		t.Errorf("c1 should be ~50/50, got ratio %.3f, counts=%v", ratio, counts)
	}
}

// TestDMDynamicTeleportation: canonical teleportation circuit.
func TestDMDynamicTeleportation(t *testing.T) {
	c, err := builder.New("teleport", 3).WithClbits(3).
		X(0).
		H(1).
		CNOT(1, 2).
		CNOT(0, 1).
		H(0).
		Measure(0, 0).
		Measure(1, 1).
		If(1, 1, gate.X, 2).
		If(0, 1, gate.Z, 2).
		Measure(2, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	for bs, n := range counts {
		if len(bs) != 3 {
			t.Errorf("unexpected bitstring length: %q", bs)
			continue
		}
		if bs[0] != '1' {
			t.Errorf("expected c2=1 in all shots, got %q (%d times)", bs, n)
		}
	}
}

// TestDMDynamicNonDynamicFastPath ensures non-dynamic circuits use the evolve-then-sample path.
func TestDMDynamicNonDynamicFastPath(t *testing.T) {
	c, err := builder.New("bell", 2).WithClbits(2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if c.IsDynamic() {
		t.Fatal("Bell state circuit should not be dynamic")
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for bs := range counts {
		if bs != "00" && bs != "11" {
			t.Errorf("unexpected bitstring %q in Bell state", bs)
		}
	}
}
