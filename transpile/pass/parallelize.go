package pass

import (
	"sort"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

// ParallelizeOps reorders independent operations to minimize circuit depth.
func ParallelizeOps(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	if len(ops) <= 1 {
		return c, nil
	}

	n := c.NumQubits()

	// Compute the earliest layer each op can be placed in.
	layers := make([]int, len(ops))
	qubitReady := make([]int, n) // earliest available layer per qubit

	for i, op := range ops {
		earliest := 0
		for _, q := range op.Qubits {
			if q < n && qubitReady[q] > earliest {
				earliest = qubitReady[q]
			}
		}
		layers[i] = earliest
		for _, q := range op.Qubits {
			if q < n {
				qubitReady[q] = earliest + 1
			}
		}
	}

	// Sort operations by their earliest layer (stable to preserve order within layer).
	indices := make([]int, len(ops))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		return layers[indices[a]] < layers[indices[b]]
	})

	result := make([]ir.Operation, len(ops))
	for i, idx := range indices {
		result[i] = ops[idx]
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}
