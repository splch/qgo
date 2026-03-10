// Package pass provides individual transpilation passes.
package pass

import (
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

// RemoveBarriers strips barrier pseudo-gates from the circuit.
func RemoveBarriers(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	filtered := make([]ir.Operation, 0, len(ops))
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Name() == "barrier" {
			continue
		}
		filtered = append(filtered, op)
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), filtered, c.Metadata()), nil
}
