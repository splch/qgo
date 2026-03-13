package param

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestSymRX_Bind(t *testing.T) {
	theta := New("theta")
	g := SymRX(theta.Expr())

	if g.Name() != "RX({theta})" {
		t.Errorf("Name() = %q", g.Name())
	}
	if g.Qubits() != 1 {
		t.Errorf("Qubits() = %d, want 1", g.Qubits())
	}

	b, ok := g.(gate.Bindable)
	if !ok {
		t.Fatal("SymRX should implement gate.Bindable")
	}
	if b.IsBound() {
		t.Error("should not be bound")
	}
	if len(b.FreeParameters()) != 1 || b.FreeParameters()[0] != "theta" {
		t.Errorf("FreeParameters() = %v", b.FreeParameters())
	}

	concrete, err := b.Bind(map[string]float64{"theta": math.Pi / 2})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the concrete gate matches gate.RX(pi/2).
	expected := gate.RX(math.Pi / 2)
	em, cm := expected.Matrix(), concrete.Matrix()
	for i := range em {
		if cmplx.Abs(em[i]-cm[i]) > 1e-10 {
			t.Errorf("matrix[%d] = %v, want %v", i, cm[i], em[i])
		}
	}
}

func TestSymRZ_Bind(t *testing.T) {
	theta := New("theta")
	g := SymRZ(theta.Expr())

	b := g.(gate.Bindable)
	concrete, err := b.Bind(map[string]float64{"theta": math.Pi / 4})
	if err != nil {
		t.Fatal(err)
	}
	expected := gate.RZ(math.Pi / 4)
	em, cm := expected.Matrix(), concrete.Matrix()
	for i := range em {
		if cmplx.Abs(em[i]-cm[i]) > 1e-10 {
			t.Errorf("matrix[%d] = %v, want %v", i, cm[i], em[i])
		}
	}
}

func TestSymU3_Bind(t *testing.T) {
	th := New("th")
	ph := New("ph")
	lm := New("lm")
	g := SymU3(th.Expr(), ph.Expr(), lm.Expr())

	b := g.(gate.Bindable)
	params := b.FreeParameters()
	if len(params) != 3 {
		t.Errorf("FreeParameters() = %v, want 3 params", params)
	}

	concrete, err := b.Bind(map[string]float64{
		"th": math.Pi / 2,
		"ph": 0,
		"lm": math.Pi,
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := gate.U3(math.Pi/2, 0, math.Pi)
	em, cm := expected.Matrix(), concrete.Matrix()
	for i := range em {
		if cmplx.Abs(em[i]-cm[i]) > 1e-10 {
			t.Errorf("matrix[%d] = %v, want %v", i, cm[i], em[i])
		}
	}
}

func TestSymCP_Bind(t *testing.T) {
	phi := New("phi")
	g := SymCP(phi.Expr())

	if g.Qubits() != 2 {
		t.Errorf("Qubits() = %d, want 2", g.Qubits())
	}

	b := g.(gate.Bindable)
	concrete, err := b.Bind(map[string]float64{"phi": math.Pi / 3})
	if err != nil {
		t.Fatal(err)
	}
	expected := gate.CP(math.Pi / 3)
	em, cm := expected.Matrix(), concrete.Matrix()
	for i := range em {
		if cmplx.Abs(em[i]-cm[i]) > 1e-10 {
			t.Errorf("matrix[%d] = %v, want %v", i, cm[i], em[i])
		}
	}
}

func TestSymbolicGate_MatrixPanics(t *testing.T) {
	g := SymRX(New("x").Expr())
	defer func() {
		if r := recover(); r == nil {
			t.Error("Matrix() should panic on unbound gate")
		}
	}()
	g.Matrix()
}

func TestSymbolicGate_BindUnbound(t *testing.T) {
	g := SymRX(New("x").Expr())
	b := g.(gate.Bindable)
	_, err := b.Bind(map[string]float64{"y": 1.0}) // wrong name
	if err == nil {
		t.Error("expected error for unbound parameter")
	}
}

func TestSymbolicGate_Inverse(t *testing.T) {
	g := SymRX(New("theta").Expr())
	inv := g.Inverse()
	if inv.Name() != "RX†({(-theta)})" {
		t.Errorf("Inverse Name() = %q", inv.Name())
	}
}

func TestCircuitBind(t *testing.T) {
	theta := New("theta")
	phi := New("phi")

	// Build a symbolic circuit.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: SymRZ(theta.Expr()), Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: SymRX(phi.Expr()), Qubits: []int{1}},
	}
	c := ir.New("sym", 2, 0, ops, nil)

	// Check free parameters.
	free := ir.FreeParameters(c)
	if len(free) != 2 {
		t.Fatalf("FreeParameters() = %v, want 2 params", free)
	}

	// Bind.
	bound, err := ir.Bind(c, map[string]float64{
		"theta": math.Pi / 4,
		"phi":   math.Pi / 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify bound circuit has no free params.
	if len(ir.FreeParameters(bound)) != 0 {
		t.Error("bound circuit should have no free parameters")
	}

	// Verify ops: H and CNOT should be unchanged, RZ and RX should be concrete.
	boundOps := bound.Ops()
	if boundOps[0].Gate != gate.H {
		t.Error("H gate should be preserved")
	}
	if boundOps[2].Gate != gate.CNOT {
		t.Error("CNOT gate should be preserved")
	}
	// RZ should now be a concrete gate with params.
	rzParams := boundOps[1].Gate.Params()
	if len(rzParams) != 1 || math.Abs(rzParams[0]-math.Pi/4) > 1e-10 {
		t.Errorf("RZ params = %v, want [pi/4]", rzParams)
	}
	rxParams := boundOps[3].Gate.Params()
	if len(rxParams) != 1 || math.Abs(rxParams[0]-math.Pi/2) > 1e-10 {
		t.Errorf("RX params = %v, want [pi/2]", rxParams)
	}
}

func TestCircuitBind_WithExpressions(t *testing.T) {
	theta := New("theta")
	// Use 2*theta as the RX angle.
	expr := Mul(Literal(2), theta.Expr())

	ops := []ir.Operation{
		{Gate: SymRX(expr), Qubits: []int{0}},
	}
	c := ir.New("expr", 1, 0, ops, nil)

	bound, err := ir.Bind(c, map[string]float64{"theta": math.Pi / 4})
	if err != nil {
		t.Fatal(err)
	}

	// Should produce RX(pi/2).
	p := bound.Ops()[0].Gate.Params()
	if len(p) != 1 || math.Abs(p[0]-math.Pi/2) > 1e-10 {
		t.Errorf("params = %v, want [pi/2]", p)
	}
}
