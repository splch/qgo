package pass

import (
	"math"
	"strings"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/analysis"
	"github.com/splch/qgo/transpile/target"
)

// MergeRotations merges consecutive same-axis rotations on the same qubit.
// RZ(a)·RZ(b) → RZ(a+b), etc. Removes gates where merged angle ≈ 0 mod 2π.
func MergeRotations(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	for {
		merged := false
		ops, merged = mergeOnce(ops, c.NumQubits(), c.NumClbits())
		if !merged {
			break
		}
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), ops, c.Metadata()), nil
}

func mergeOnce(ops []ir.Operation, numQubits, numClbits int) ([]ir.Operation, bool) {
	if len(ops) == 0 {
		return ops, false
	}

	tmp := ir.New("", numQubits, numClbits, ops, nil)
	timelines := analysis.BuildTimelines(tmp)

	removed := make([]bool, len(ops))
	replacement := make(map[int]ir.Operation)
	merged := false

	for i := range ops {
		if removed[i] || ops[i].Gate == nil {
			continue
		}
		op := ops[i]
		if op.Gate.Qubits() != 1 || op.Gate.Params() == nil || len(op.Gate.Params()) != 1 {
			continue
		}

		axis := rotationAxis(op.Gate)
		if axis == "" {
			continue
		}

		// Find next single-qubit rotation on same qubit with same axis.
		q := op.Qubits[0]
		j := analysis.NextOnQubit(timelines, q, i)
		for j >= 0 && removed[j] {
			j = analysis.NextOnQubit(timelines, q, j)
		}
		if j < 0 {
			continue
		}
		next := ops[j]
		if next.Gate == nil || next.Gate.Qubits() != 1 || next.Gate.Params() == nil || len(next.Gate.Params()) != 1 {
			continue
		}
		if rotationAxis(next.Gate) != axis {
			continue
		}
		if !sameQubits(op.Qubits, next.Qubits) {
			continue
		}

		// Merge: sum angles.
		angle := op.Gate.Params()[0] + next.Gate.Params()[0]
		angle = normalizeAngle(angle)

		if nearZeroMod2Pi(angle) {
			// Both cancel out.
			removed[i] = true
			removed[j] = true
		} else {
			// Replace first with merged, remove second.
			var newGate gate.Gate
			switch axis {
			case "RZ":
				newGate = gate.RZ(angle)
			case "RY":
				newGate = gate.RY(angle)
			case "RX":
				newGate = gate.RX(angle)
			}
			replacement[i] = ir.Operation{Gate: newGate, Qubits: op.Qubits}
			removed[j] = true
		}
		merged = true
	}

	if !merged {
		return ops, false
	}

	var result []ir.Operation
	for i, op := range ops {
		if removed[i] {
			continue
		}
		if rep, ok := replacement[i]; ok {
			result = append(result, rep)
		} else {
			result = append(result, op)
		}
	}
	return result, true
}

// rotationAxis returns "RX", "RY", or "RZ" for rotation gates, or "" otherwise.
func rotationAxis(g gate.Gate) string {
	name := g.Name()
	if strings.HasPrefix(name, "RZ") {
		return "RZ"
	}
	if strings.HasPrefix(name, "RY") {
		return "RY"
	}
	if strings.HasPrefix(name, "RX") {
		return "RX"
	}
	return ""
}

func normalizeAngle(angle float64) float64 {
	a := math.Mod(angle, 2*math.Pi)
	if a > math.Pi {
		a -= 2 * math.Pi
	} else if a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func nearZeroMod2Pi(angle float64) bool {
	a := math.Mod(angle, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a < 1e-10 || (2*math.Pi-a) < 1e-10
}
