package gate

import (
	"math"
	"math/cmplx"
	"strings"
	"testing"
)

func TestUnitaryValidCreation(t *testing.T) {
	s2 := 1.0 / math.Sqrt2

	tests := []struct {
		name   string
		matrix []complex128
		qubits int
	}{
		{
			name: "H-like",
			matrix: []complex128{
				complex(s2, 0), complex(s2, 0),
				complex(s2, 0), complex(-s2, 0),
			},
			qubits: 1,
		},
		{
			name: "CNOT-like",
			matrix: []complex128{
				1, 0, 0, 0,
				0, 1, 0, 0,
				0, 0, 0, 1,
				0, 0, 1, 0,
			},
			qubits: 2,
		},
		{
			name: "CCX-like",
			matrix: []complex128{
				1, 0, 0, 0, 0, 0, 0, 0,
				0, 1, 0, 0, 0, 0, 0, 0,
				0, 0, 1, 0, 0, 0, 0, 0,
				0, 0, 0, 1, 0, 0, 0, 0,
				0, 0, 0, 0, 1, 0, 0, 0,
				0, 0, 0, 0, 0, 1, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 1,
				0, 0, 0, 0, 0, 0, 1, 0,
			},
			qubits: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := Unitary(tc.name, tc.matrix)
			if err != nil {
				t.Fatalf("Unitary(%q) error: %v", tc.name, err)
			}
			if g.Name() != tc.name {
				t.Errorf("Name() = %q, want %q", g.Name(), tc.name)
			}
			if g.Qubits() != tc.qubits {
				t.Errorf("Qubits() = %d, want %d", g.Qubits(), tc.qubits)
			}
			if g.Params() != nil {
				t.Errorf("Params() = %v, want nil", g.Params())
			}
			if g.Decompose(nil) != nil {
				t.Errorf("Decompose() = %v, want nil", g.Decompose(nil))
			}
		})
	}
}

func TestUnitaryInvalidMatrix(t *testing.T) {
	tests := []struct {
		name    string
		matrix  []complex128
		wantErr string
	}{
		{
			name:    "non-unitary",
			matrix:  []complex128{1, 1, 1, 1},
			wantErr: "not unitary",
		},
		{
			name:    "wrong length 9",
			matrix:  make([]complex128, 9),
			wantErr: "matrix length 9 invalid",
		},
		{
			name:    "wrong length 0",
			matrix:  nil,
			wantErr: "matrix length 0 invalid",
		},
		{
			name:    "wrong length 3",
			matrix:  []complex128{1, 0, 0},
			wantErr: "matrix length 3 invalid",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Unitary("bad", tc.matrix)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestUnitaryInverse(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	hMatrix := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}

	g, err := Unitary("myH", hMatrix)
	if err != nil {
		t.Fatalf("Unitary error: %v", err)
	}

	inv := g.Inverse()

	// Inverse name should have dagger.
	if inv.Name() != "myH†" {
		t.Errorf("Inverse().Name() = %q, want %q", inv.Name(), "myH†")
	}

	// Verify g * g.Inverse() = I
	dim := 1 << g.Qubits()
	m := g.Matrix()
	mi := inv.Matrix()
	for r := range dim {
		for c := range dim {
			var sum complex128
			for k := range dim {
				sum += m[r*dim+k] * mi[k*dim+c]
			}
			expected := complex(0, 0)
			if r == c {
				expected = 1
			}
			if cmplx.Abs(sum-expected) > eps {
				t.Errorf("(g*g†)[%d,%d] = %v, want %v", r, c, sum, expected)
			}
		}
	}

	// The inverse should also be a valid unitary.
	assertUnitary(t, inv)
}

func TestUnitaryDefensiveCopy(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	matrix := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}

	g, err := Unitary("copyTest", matrix)
	if err != nil {
		t.Fatalf("Unitary error: %v", err)
	}

	// Mutate the original input.
	matrix[0] = 999

	// Gate matrix should be unchanged.
	m := g.Matrix()
	if cmplx.Abs(m[0]-complex(s2, 0)) > eps {
		t.Errorf("Matrix[0] = %v after input mutation, want %v", m[0], complex(s2, 0))
	}
}

func TestUnitaryNameAndQubits(t *testing.T) {
	g, err := Unitary("identity2", []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "identity2" {
		t.Errorf("Name() = %q, want %q", g.Name(), "identity2")
	}
	if g.Qubits() != 2 {
		t.Errorf("Qubits() = %d, want 2", g.Qubits())
	}
}

func TestMustUnitaryPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from MustUnitary with invalid matrix")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "gate.MustUnitary") {
			t.Errorf("panic message = %q, want substring %q", msg, "gate.MustUnitary")
		}
	}()
	MustUnitary("bad", []complex128{1, 1, 1, 1})
}

func TestMustUnitaryValid(t *testing.T) {
	// Should not panic.
	g := MustUnitary("X-custom", []complex128{0, 1, 1, 0})
	if g.Name() != "X-custom" {
		t.Errorf("Name() = %q, want %q", g.Name(), "X-custom")
	}
}

func TestUnitaryUnitarityCheck(t *testing.T) {
	// Use existing standard gates as source of known-good unitary matrices.
	gates := []Gate{H, X, Y, Z, S, T, CNOT, CZ, SWAP, CCX, CSWAP}
	for _, g := range gates {
		t.Run(g.Name(), func(t *testing.T) {
			u, err := Unitary("custom-"+g.Name(), g.Matrix())
			if err != nil {
				t.Fatalf("Unitary from %s matrix: %v", g.Name(), err)
			}
			assertUnitary(t, u)
		})
	}
}
