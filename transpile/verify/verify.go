// Package verify provides statevector-based circuit equivalence checking.
package verify

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// EquivalentOnZero checks if two circuits produce equivalent statevectors
// when starting from |0...0>, up to a global phase.
func EquivalentOnZero(a, b *ir.Circuit, tol float64) (bool, error) {
	n := a.NumQubits()
	if b.NumQubits() != n {
		return false, fmt.Errorf("verify: qubit count mismatch: %d vs %d", n, b.NumQubits())
	}
	if n > 14 {
		return false, fmt.Errorf("verify: circuit too large for equivalence check: %d qubits", n)
	}

	simA := statevector.New(n)
	if err := simA.Evolve(a); err != nil {
		return false, fmt.Errorf("verify: evolving circuit A: %w", err)
	}
	svA := simA.StateVector()

	simB := statevector.New(n)
	if err := simB.Evolve(b); err != nil {
		return false, fmt.Errorf("verify: evolving circuit B: %w", err)
	}
	svB := simB.StateVector()

	return stateVectorsClose(svA, svB, tol), nil
}

// Equivalent checks if two circuits have the same unitary up to global phase
// by testing on all computational basis states.
func Equivalent(a, b *ir.Circuit, tol float64) (bool, error) {
	n := a.NumQubits()
	if b.NumQubits() != n {
		return false, fmt.Errorf("verify: qubit count mismatch: %d vs %d", n, b.NumQubits())
	}
	if n > 10 {
		return false, fmt.Errorf("verify: circuit too large for full equivalence check: %d qubits", n)
	}

	dim := 1 << n

	// Build unitaries column by column.
	unitaryA := make([][]complex128, dim)
	unitaryB := make([][]complex128, dim)

	for col := range dim {
		// Prepend X gates to prepare basis state |col>, then apply circuit.
		prepA := prependBasisPrep(a, col)
		prepB := prependBasisPrep(b, col)

		simA := statevector.New(n)
		simB := statevector.New(n)

		if err := simA.Evolve(prepA); err != nil {
			return false, err
		}
		if err := simB.Evolve(prepB); err != nil {
			return false, err
		}

		unitaryA[col] = simA.StateVector()
		unitaryB[col] = simB.StateVector()
	}

	// Compare unitaries up to global phase.
	return unitariesClose(unitaryA, unitaryB, dim, tol), nil
}

// prependBasisPrep creates a new circuit with X gates prepended to prepare |idx>.
func prependBasisPrep(c *ir.Circuit, idx int) *ir.Circuit {
	n := c.NumQubits()
	var ops []ir.Operation
	for q := range n {
		if idx&(1<<q) != 0 {
			ops = append(ops, ir.Operation{Gate: gate.X, Qubits: []int{q}})
		}
	}
	ops = append(ops, c.Ops()...)
	return ir.New(c.Name(), n, c.NumClbits(), ops, c.Metadata())
}

// stateVectorsClose checks if two statevectors are equal up to global phase.
func stateVectorsClose(a, b []complex128, tol float64) bool {
	if len(a) != len(b) {
		return false
	}

	// Find the global phase from the first non-negligible element.
	var phase complex128
	found := false
	for i := range a {
		if cmplx.Abs(b[i]) > tol {
			if cmplx.Abs(a[i]) < tol {
				return false
			}
			phase = a[i] / b[i]
			if math.Abs(cmplx.Abs(phase)-1) > tol {
				return false
			}
			found = true
			break
		} else if cmplx.Abs(a[i]) > tol {
			return false
		}
	}
	if !found {
		return true // both zero
	}

	// Verify all elements match with this phase.
	for i := range a {
		if cmplx.Abs(a[i]-phase*b[i]) > tol {
			return false
		}
	}
	return true
}

// unitariesClose checks if two unitaries (stored as column vectors) are equal up to global phase.
func unitariesClose(a, b [][]complex128, dim int, tol float64) bool {
	// Find global phase from first non-negligible element in column 0.
	var phase complex128
	found := false
	for i := range dim {
		if cmplx.Abs(b[0][i]) > tol {
			if cmplx.Abs(a[0][i]) < tol {
				return false
			}
			phase = a[0][i] / b[0][i]
			found = true
			break
		}
	}
	if !found {
		return true
	}

	// Check all columns.
	for col := range dim {
		for row := range dim {
			if cmplx.Abs(a[col][row]-phase*b[col][row]) > tol {
				return false
			}
		}
	}
	return true
}
