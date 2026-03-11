package pass

import (
	"fmt"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/internal/mathutil"
	"github.com/splch/qgo/transpile/target"
)

// FixDirection corrects 2-qubit gate directions for targets with asymmetric connectivity.
// For CX/CNOT: uses H-conjugation to reverse direction.
// For CZ, SWAP, CP: swaps operands (symmetric gates).
func FixDirection(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	if t.Connectivity == nil {
		return c, nil // all-to-all, no direction constraints
	}

	var result []ir.Operation
	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Qubits() != 2 {
			result = append(result, op)
			continue
		}

		q0, q1 := op.Qubits[0], op.Qubits[1]

		// Already in native direction.
		if t.HasDirection(q0, q1) {
			result = append(result, op)
			continue
		}

		// Check reverse direction.
		if !t.HasDirection(q1, q0) {
			return nil, fmt.Errorf("fix_direction: no connectivity between qubits %d and %d", q0, q1)
		}

		// Gate-specific reversal.
		bname := mathutil.StripParams(op.Gate.Name())
		switch bname {
		case "CZ", "SWAP":
			// Symmetric gates: just swap operands.
			result = append(result, ir.Operation{
				Gate:   op.Gate,
				Qubits: []int{q1, q0},
			})
		case "CP":
			// CP is symmetric: diag(1,1,1,exp(i*phi)).
			result = append(result, ir.Operation{
				Gate:   op.Gate,
				Qubits: []int{q1, q0},
			})
		case "CNOT", "CX":
			// CX reversal: H(q0) H(q1) CX(q1,q0) H(q1) H(q0).
			result = append(result,
				ir.Operation{Gate: gate.H, Qubits: []int{q0}},
				ir.Operation{Gate: gate.H, Qubits: []int{q1}},
				ir.Operation{Gate: gate.CNOT, Qubits: []int{q1, q0}},
				ir.Operation{Gate: gate.H, Qubits: []int{q1}},
				ir.Operation{Gate: gate.H, Qubits: []int{q0}},
			)
		default:
			return nil, fmt.Errorf("fix_direction: unsupported gate %q for direction reversal (decompose to CX first)", op.Gate.Name())
		}
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}
