package param

import (
	"math"
	"testing"
)

func TestLiteral(t *testing.T) {
	e := Literal(3.14)
	v, err := e.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != 3.14 {
		t.Errorf("Eval() = %v, want 3.14", v)
	}
	if !e.IsNumeric() {
		t.Error("Literal should be numeric")
	}
	if len(e.Parameters()) != 0 {
		t.Error("Literal should have no parameters")
	}
}

func TestParamRef(t *testing.T) {
	p := New("theta")
	e := p.Expr()

	if e.IsNumeric() {
		t.Error("paramRef should not be numeric")
	}
	if len(e.Parameters()) != 1 || e.Parameters()[0] != p {
		t.Error("Parameters() should return [theta]")
	}
	if e.String() != "theta" {
		t.Errorf("String() = %q, want %q", e.String(), "theta")
	}

	v, err := e.Eval(map[string]float64{"theta": 1.5})
	if err != nil {
		t.Fatal(err)
	}
	if v != 1.5 {
		t.Errorf("Eval() = %v, want 1.5", v)
	}

	_, err = e.Eval(map[string]float64{})
	if err == nil {
		t.Error("expected error for unbound parameter")
	}
}

func TestArithmetic(t *testing.T) {
	a := New("a")
	b := New("b")
	bindings := map[string]float64{"a": 3, "b": 2}

	tests := []struct {
		name string
		expr Expr
		want float64
	}{
		{"add", Add(a.Expr(), b.Expr()), 5},
		{"sub", Sub(a.Expr(), b.Expr()), 1},
		{"mul", Mul(a.Expr(), b.Expr()), 6},
		{"div", Div(a.Expr(), b.Expr()), 1.5},
		{"neg", Neg(a.Expr()), -3},
		{"compound", Add(Mul(a.Expr(), Literal(2)), b.Expr()), 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.expr.Eval(bindings)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(v-tt.want) > 1e-10 {
				t.Errorf("Eval() = %v, want %v", v, tt.want)
			}
		})
	}
}

func TestDivByZero(t *testing.T) {
	e := Div(Literal(1), Literal(0))
	_, err := e.Eval(nil)
	if err == nil {
		t.Error("expected division by zero error")
	}
}

func TestIsNumeric(t *testing.T) {
	p := New("x")
	if Add(Literal(1), Literal(2)).IsNumeric() != true {
		t.Error("Literal + Literal should be numeric")
	}
	if Add(Literal(1), p.Expr()).IsNumeric() != false {
		t.Error("Literal + param should not be numeric")
	}
}

func TestMergeParams(t *testing.T) {
	a := New("a")
	b := New("b")

	e := Add(a.Expr(), Add(a.Expr(), b.Expr()))
	params := e.Parameters()
	if len(params) != 2 {
		t.Errorf("Parameters() = %d params, want 2 (deduplicated)", len(params))
	}
}
