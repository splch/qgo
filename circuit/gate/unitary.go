package gate

import (
	"fmt"
	"math"
)

// unitary is a gate defined by a user-provided unitary matrix.
type unitary struct {
	name   string
	n      int             // 1, 2, or 3 qubits
	matrix []complex128    // defensive copy
}

func (g *unitary) Name() string         { return g.name }
func (g *unitary) Qubits() int          { return g.n }
func (g *unitary) Matrix() []complex128  { return g.matrix }
func (g *unitary) Params() []float64    { return nil }
func (g *unitary) Decompose(_ []int) []Applied { return nil }

func (g *unitary) Inverse() Gate {
	dim := 1 << g.n
	inv := make([]complex128, dim*dim)
	for r := range dim {
		for c := range dim {
			inv[r*dim+c] = conj(g.matrix[c*dim+r])
		}
	}
	return &unitary{name: g.name + "†", n: g.n, matrix: inv}
}

// Unitary creates a gate from a user-provided unitary matrix.
// The name is used for display. The matrix must be 2x2, 4x4, or 8x8
// (flat row-major []complex128 of length 4, 16, or 64).
// Returns an error if the matrix is not unitary (U†U ≈ I within 1e-10 tolerance).
func Unitary(name string, matrix []complex128) (Gate, error) {
	n, err := validateUnitary(matrix)
	if err != nil {
		return nil, err
	}
	m := make([]complex128, len(matrix))
	copy(m, matrix)
	return &unitary{name: name, n: n, matrix: m}, nil
}

// MustUnitary is like Unitary but panics on error.
func MustUnitary(name string, matrix []complex128) Gate {
	g, err := Unitary(name, matrix)
	if err != nil {
		panic(fmt.Sprintf("gate.MustUnitary: %v", err))
	}
	return g
}

func validateUnitary(matrix []complex128) (int, error) {
	var n int
	switch len(matrix) {
	case 4:
		n = 1
	case 16:
		n = 2
	case 64:
		n = 3
	default:
		return 0, fmt.Errorf("gate.Unitary: matrix length %d invalid (must be 4, 16, or 64)", len(matrix))
	}
	dim := 1 << n
	// Check U†U ≈ I
	const tol = 1e-10
	for i := range dim {
		for j := range dim {
			// (U†U)[i][j] = sum_k conj(U[k][i]) * U[k][j]
			var sum complex128
			for k := range dim {
				sum += conj(matrix[k*dim+i]) * matrix[k*dim+j]
			}
			expected := complex(0, 0)
			if i == j {
				expected = 1
			}
			diff := sum - expected
			if math.Abs(real(diff)) > tol || math.Abs(imag(diff)) > tol {
				return 0, fmt.Errorf("gate.Unitary: matrix is not unitary (U†U)[%d][%d] = %v, want %v", i, j, sum, expected)
			}
		}
	}
	return n, nil
}
