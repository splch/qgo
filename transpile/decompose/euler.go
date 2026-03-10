package decompose

import (
	"math"
	"math/cmplx"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// EulerZYZ decomposes a 2×2 unitary U into Rz(alpha)·Ry(beta)·Rz(gamma)
// plus a global phase: U = e^{i·phase} · Rz(alpha) · Ry(beta) · Rz(gamma).
func EulerZYZ(m []complex128) (alpha, beta, gamma, phase float64) {
	// Normalize to SU(2).
	det := Det2x2(m)
	detPhase := cmplx.Phase(det) / 2
	factor := cmplx.Exp(complex(0, -detPhase))
	// su2 = m * e^{-i*detPhase}
	a := m[0] * factor // cos(beta/2) * e^{i(alpha+gamma)/2}
	b := m[1] * factor // -sin(beta/2) * e^{i(alpha-gamma)/2}

	absB := cmplx.Abs(b)
	beta = 2 * math.Acos(clamp(cmplx.Abs(a), 0, 1))

	if absB < 1e-10 {
		// Near identity: Rz(alpha+gamma). Assign all to alpha.
		// a = e^{-i(α+γ)/2}, so Phase(a) = -(α+γ)/2, thus α = -2·Phase(a).
		alpha = -2 * cmplx.Phase(a)
		beta = 0
		gamma = 0
	} else if cmplx.Abs(a) < 1e-10 {
		// beta ≈ π: cos(β/2) ≈ 0, so use b.
		// b = -sin(β/2)·e^{-i(α-γ)/2}, Phase(-b) = -(α-γ)/2
		alpha = -2 * cmplx.Phase(-b)
		beta = math.Pi
		gamma = 0
	} else {
		// General case.
		// a = cos(β/2) · e^{-i(α+γ)/2}  →  Phase(a) = -(α+γ)/2
		// -b = sin(β/2) · e^{-i(α-γ)/2}  →  Phase(-b) = -(α-γ)/2
		apg := cmplx.Phase(a)  // -(alpha+gamma)/2
		amg := cmplx.Phase(-b) // -(alpha-gamma)/2
		alpha = -(apg + amg)
		gamma = -(apg - amg)
	}

	phase = detPhase
	return
}

// EulerDecompose returns the gate sequence for a single-qubit gate on the given qubit,
// decomposed as Rz(alpha)·Ry(beta)·Rz(gamma). Skips identity rotations.
func EulerDecompose(g gate.Gate, qubit int) []ir.Operation {
	if g.Qubits() != 1 {
		return nil
	}
	alpha, beta, gamma, _ := EulerZYZ(g.Matrix())

	var ops []ir.Operation
	if !nearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RZ(normalizeAngle(gamma)),
			Qubits: []int{qubit},
		})
	}
	if !nearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RY(normalizeAngle(beta)),
			Qubits: []int{qubit},
		})
	}
	if !nearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RZ(normalizeAngle(alpha)),
			Qubits: []int{qubit},
		})
	}

	if len(ops) == 0 {
		// Gate is effectively identity; no operations needed.
		return nil
	}
	return ops
}

// clamp restricts v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// nearZeroMod2Pi reports whether angle is ≈ 0 mod 2π.
func nearZeroMod2Pi(angle float64) bool {
	a := math.Mod(angle, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a < 1e-10 || (2*math.Pi-a) < 1e-10
}

// normalizeAngle wraps angle to (-π, π].
func normalizeAngle(angle float64) float64 {
	a := math.Mod(angle, 2*math.Pi)
	if a > math.Pi {
		a -= 2 * math.Pi
	} else if a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
