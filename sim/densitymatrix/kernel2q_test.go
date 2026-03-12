package densitymatrix

import (
	"math"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/sim/statevector"
)

func TestGate2ColsParallel_CNOT9Q(t *testing.T) {
	// 9 qubits triggers parallel code path (>= parallelThreshold).
	// H(0) + CNOT(0,1) creates a Bell pair; verify fidelity matches statevector.
	c, err := builder.New("bell9", 9).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(9)
	if err := sv.Evolve(c); err != nil {
		t.Fatal(err)
	}

	dm := New(9)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	f := dm.Fidelity(sv.StateVector())
	if math.Abs(f-1.0) > 1e-8 {
		t.Errorf("CNOT 9Q fidelity = %f, want 1.0", f)
	}
}

func TestGate2ColsParallel_CZ10Q(t *testing.T) {
	// CZ is another asymmetric 2Q gate; 10 qubits ensures parallel path.
	c, err := builder.New("cz10", 10).H(0).X(1).CZ(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(10)
	if err := sv.Evolve(c); err != nil {
		t.Fatal(err)
	}

	dm := New(10)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	f := dm.Fidelity(sv.StateVector())
	if math.Abs(f-1.0) > 1e-8 {
		t.Errorf("CZ 10Q fidelity = %f, want 1.0", f)
	}
}

func TestGate2ColsParallel_GHZ9Q(t *testing.T) {
	// 9-qubit GHZ: H + 8 CNOTs exercises the parallel 2Q path heavily.
	b := builder.New("ghz9", 9)
	b.H(0)
	for i := range 8 {
		b.CNOT(i, i+1)
	}
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(9)
	if err := sv.Evolve(c); err != nil {
		t.Fatal(err)
	}

	dm := New(9)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	f := dm.Fidelity(sv.StateVector())
	if math.Abs(f-1.0) > 1e-8 {
		t.Errorf("GHZ 9Q fidelity = %f, want 1.0", f)
	}
}
