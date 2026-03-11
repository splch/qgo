package decompose

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// rzMatrix returns the 2x2 RZ(theta) matrix as flat row-major []complex128.
func rzMatrix(theta float64) []complex128 {
	return []complex128{
		cmplx.Exp(complex(0, -theta/2)), 0,
		0, cmplx.Exp(complex(0, theta/2)),
	}
}

// ryMatrix returns the 2x2 RY(theta) matrix as flat row-major []complex128.
func ryMatrix(theta float64) []complex128 {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return []complex128{
		complex(c, 0), complex(-s, 0),
		complex(s, 0), complex(c, 0),
	}
}

// reconstructZYZ builds the matrix Rz(alpha) * Ry(beta) * Rz(gamma).
func reconstructZYZ(alpha, beta, gamma float64) []complex128 {
	rza := rzMatrix(alpha)
	ryb := ryMatrix(beta)
	rzg := rzMatrix(gamma)
	// Rz(alpha) * Ry(beta)
	tmp := MatMul(rza, ryb, 2)
	// (Rz(alpha) * Ry(beta)) * Rz(gamma)
	return MatMul(tmp, rzg, 2)
}

// assertEulerZYZ checks that EulerZYZ decomposes the gate matrix correctly:
// the reconstructed Rz(alpha)*Ry(beta)*Rz(gamma) should equal the original
// matrix up to a global phase.
func assertEulerZYZ(t *testing.T, name string, m []complex128, tol float64) {
	t.Helper()
	alpha, beta, gamma, _ := EulerZYZ(m)
	got := reconstructZYZ(alpha, beta, gamma)
	if _, ok := GlobalPhase(got, m, tol); !ok {
		t.Errorf("EulerZYZ(%s): reconstructed matrix does not match original up to global phase\n"+
			"  alpha=%.6f beta=%.6f gamma=%.6f\n"+
			"  original=%v\n"+
			"  reconstructed=%v",
			name, alpha, beta, gamma, m, got)
	}
}

func TestEulerZYZ_FixedGates(t *testing.T) {
	fixed := []struct {
		name string
		gate gate.Gate
	}{
		{"I", gate.I},
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"Z", gate.Z},
		{"S", gate.S},
		{"Sdg", gate.Sdg},
		{"T", gate.T},
		{"Tdg", gate.Tdg},
		{"SX", gate.SX},
	}
	for _, tc := range fixed {
		t.Run(tc.name, func(t *testing.T) {
			assertEulerZYZ(t, tc.name, tc.gate.Matrix(), 1e-10)
		})
	}
}

func TestEulerZYZ_ParameterizedGates(t *testing.T) {
	cases := []struct {
		name string
		gate gate.Gate
	}{
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"RZ(pi/6)", gate.RZ(math.Pi / 6)},
		{"U3(1.0,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use 1e-7 tolerance: EulerZYZ can introduce small numerical noise
			// in the off-diagonal elements for near-diagonal matrices (e.g., RZ).
			assertEulerZYZ(t, tc.name, tc.gate.Matrix(), 1e-7)
		})
	}
}

func TestEulerZYZ_PhaseReturned(t *testing.T) {
	// For a gate like Z (det=-1), the phase should be non-zero.
	_, _, _, phase := EulerZYZ(gate.Z.Matrix())
	// Z has det = -1, so detPhase = pi/2 (since det = e^{2i*phase} for SU(2) normalization).
	// We just check it is not exactly zero.
	if math.Abs(phase) < 1e-14 {
		t.Errorf("EulerZYZ(Z): expected non-zero phase, got %.15f", phase)
	}
}

func TestEulerZYZ_IdentityAngles(t *testing.T) {
	// Identity should give alpha=0, beta=0, gamma=0 (or equivalent mod 2pi).
	alpha, beta, gamma, _ := EulerZYZ(gate.I.Matrix())
	if !nearZeroMod2Pi(alpha) || !nearZeroMod2Pi(beta) || !nearZeroMod2Pi(gamma) {
		t.Errorf("EulerZYZ(I): expected all angles near zero mod 2pi, got alpha=%.6f beta=%.6f gamma=%.6f",
			alpha, beta, gamma)
	}
}

func TestEulerDecompose_Identity(t *testing.T) {
	// Identity should produce nil (no operations needed).
	ops := EulerDecompose(gate.I, 0)
	if ops != nil {
		t.Errorf("EulerDecompose(I): expected nil, got %d ops", len(ops))
	}
}

func TestEulerDecompose_FixedGates(t *testing.T) {
	fixed := []struct {
		name string
		gate gate.Gate
	}{
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"Z", gate.Z},
		{"S", gate.S},
		{"Sdg", gate.Sdg},
		{"T", gate.T},
		{"Tdg", gate.Tdg},
		{"SX", gate.SX},
	}
	for _, tc := range fixed {
		t.Run(tc.name, func(t *testing.T) {
			ops := EulerDecompose(tc.gate, 0)
			if len(ops) == 0 {
				t.Fatalf("EulerDecompose(%s): returned no operations", tc.name)
			}
			// Each op should be an RZ or RY on qubit 0.
			for i, op := range ops {
				if len(op.Qubits) != 1 || op.Qubits[0] != 0 {
					t.Errorf("EulerDecompose(%s): op[%d] has unexpected qubits %v", tc.name, i, op.Qubits)
				}
				name := op.Gate.Name()
				if len(name) < 2 || (name[:2] != "RZ" && name[:2] != "RY") {
					t.Errorf("EulerDecompose(%s): op[%d] has unexpected gate %s", tc.name, i, name)
				}
			}
			// Verify the product of the decomposed ops matches the original.
			product := Eye(2)
			for _, op := range ops {
				product = MatMul(op.Gate.Matrix(), product, 2)
			}
			if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-10); !ok {
				t.Errorf("EulerDecompose(%s): product of ops does not match original up to global phase", tc.name)
			}
		})
	}
}

func TestEulerDecompose_ParameterizedGates(t *testing.T) {
	cases := []struct {
		name string
		gate gate.Gate
	}{
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"RZ(pi/6)", gate.RZ(math.Pi / 6)},
		{"U3(1.0,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := EulerDecompose(tc.gate, 3)
			if len(ops) == 0 {
				t.Fatalf("EulerDecompose(%s): returned no operations", tc.name)
			}
			for _, op := range ops {
				if len(op.Qubits) != 1 || op.Qubits[0] != 3 {
					t.Errorf("EulerDecompose(%s): expected qubit 3, got %v", tc.name, op.Qubits)
				}
			}
			// Verify product matches original (relaxed tolerance for numerical precision).
			product := Eye(2)
			for _, op := range ops {
				product = MatMul(op.Gate.Matrix(), product, 2)
			}
			if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-7); !ok {
				t.Errorf("EulerDecompose(%s): product of ops does not match original up to global phase", tc.name)
			}
		})
	}
}

func TestEulerDecompose_RejectsMultiQubit(t *testing.T) {
	ops := EulerDecompose(gate.CNOT, 0)
	if ops != nil {
		t.Errorf("EulerDecompose(CNOT): expected nil for multi-qubit gate, got %d ops", len(ops))
	}
}

func TestEulerZYZ_MatCloseReconstruction(t *testing.T) {
	// Test using MatClose directly (not GlobalPhase) by including the phase factor.
	gates := []struct {
		name string
		gate gate.Gate
	}{
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"U3(1.0,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	for _, tc := range gates {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.gate.Matrix()
			alpha, beta, gamma, phase := EulerZYZ(m)
			got := reconstructZYZ(alpha, beta, gamma)
			// Apply phase: original = e^{i*phase} * Rz(a)*Ry(b)*Rz(g)
			phaseFactor := cmplx.Exp(complex(0, phase))
			scaled := MatScale(got, phaseFactor)
			if !MatClose(scaled, m, 1e-10) {
				t.Errorf("EulerZYZ(%s): e^{i*phase}*Rz*Ry*Rz does not match original via MatClose\n"+
					"  alpha=%.6f beta=%.6f gamma=%.6f phase=%.6f",
					tc.name, alpha, beta, gamma, phase)
			}
		})
	}
}

func TestEulerDecompose_MaxOps(t *testing.T) {
	// EulerDecompose should produce at most 3 operations (RZ, RY, RZ).
	gates := []gate.Gate{
		gate.H, gate.X, gate.Y, gate.Z, gate.S, gate.Sdg, gate.T, gate.Tdg, gate.SX,
		gate.RX(math.Pi / 3), gate.RY(math.Pi / 4), gate.RZ(math.Pi / 6),
		gate.U3(1.0, 0.5, 0.3),
	}
	for _, g := range gates {
		ops := EulerDecompose(g, 0)
		if len(ops) > 3 {
			t.Errorf("EulerDecompose(%s): produced %d ops, expected at most 3", g.Name(), len(ops))
		}
	}
}

// --- ZXZ tests ---

// rxMatrix returns the 2x2 RX(theta) matrix.
func rxMatrix(theta float64) []complex128 {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return []complex128{
		complex(c, 0), complex(0, -s),
		complex(0, -s), complex(c, 0),
	}
}

// reconstructZXZ builds the matrix Rz(alpha) * Rx(beta) * Rz(gamma).
func reconstructZXZ(alpha, beta, gamma float64) []complex128 {
	return MatMul(rzMatrix(alpha), MatMul(rxMatrix(beta), rzMatrix(gamma), 2), 2)
}

func assertEulerZXZ(t *testing.T, name string, m []complex128, tol float64) {
	t.Helper()
	alpha, beta, gamma, _ := EulerZXZ(m)
	got := reconstructZXZ(alpha, beta, gamma)
	if _, ok := GlobalPhase(got, m, tol); !ok {
		t.Errorf("EulerZXZ(%s): reconstructed matrix does not match original up to global phase\n"+
			"  alpha=%.6f beta=%.6f gamma=%.6f\n"+
			"  original=%v\n"+
			"  reconstructed=%v",
			name, alpha, beta, gamma, m, got)
	}
}

func TestEulerZXZ_FixedGates(t *testing.T) {
	fixed := []struct {
		name string
		gate gate.Gate
	}{
		{"I", gate.I},
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"Z", gate.Z},
		{"S", gate.S},
		{"Sdg", gate.Sdg},
		{"T", gate.T},
		{"Tdg", gate.Tdg},
		{"SX", gate.SX},
	}
	for _, tc := range fixed {
		t.Run(tc.name, func(t *testing.T) {
			assertEulerZXZ(t, tc.name, tc.gate.Matrix(), 1e-10)
		})
	}
}

func TestEulerZXZ_ParameterizedGates(t *testing.T) {
	cases := []struct {
		name string
		gate gate.Gate
	}{
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"RZ(pi/6)", gate.RZ(math.Pi / 6)},
		{"U3(1.0,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertEulerZXZ(t, tc.name, tc.gate.Matrix(), 1e-7)
		})
	}
}

// --- ZSX tests ---

// opsProduct multiplies the matrices of a list of operations (right-to-left application).
func opsProduct(ops []ir.Operation) []complex128 {
	product := Eye(2)
	for _, op := range ops {
		product = MatMul(op.Gate.Matrix(), product, 2)
	}
	return product
}

func TestEulerZSX_FixedGates(t *testing.T) {
	fixed := []struct {
		name string
		gate gate.Gate
	}{
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"Z", gate.Z},
		{"S", gate.S},
		{"Sdg", gate.Sdg},
		{"T", gate.T},
		{"Tdg", gate.Tdg},
		{"SX", gate.SX},
	}
	for _, tc := range fixed {
		t.Run(tc.name, func(t *testing.T) {
			ops := eulerZSX(tc.gate.Matrix(), 0)
			if len(ops) == 0 {
				t.Fatalf("eulerZSX(%s): returned no operations", tc.name)
			}
			product := opsProduct(ops)
			if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-7); !ok {
				t.Errorf("eulerZSX(%s): product does not match original up to global phase\n"+
					"  original=%v\n  product=%v", tc.name, tc.gate.Matrix(), product)
			}
		})
	}
}

func TestEulerZSX_ParameterizedGates(t *testing.T) {
	cases := []struct {
		name string
		gate gate.Gate
	}{
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"RZ(pi/6)", gate.RZ(math.Pi / 6)},
		{"U3(1.0,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := eulerZSX(tc.gate.Matrix(), 0)
			if len(ops) == 0 {
				t.Fatalf("eulerZSX(%s): returned no operations", tc.name)
			}
			product := opsProduct(ops)
			if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-7); !ok {
				t.Errorf("eulerZSX(%s): product does not match original up to global phase\n"+
					"  original=%v\n  product=%v", tc.name, tc.gate.Matrix(), product)
			}
		})
	}
}

func TestEulerZSX_SpecialCases(t *testing.T) {
	cases := []struct {
		name     string
		m        []complex128
		maxOps   int
	}{
		{"Identity", gate.I.Matrix(), 0},
		{"RZ(pi/4)", gate.RZ(math.Pi / 4).Matrix(), 1},
		{"Z", gate.Z.Matrix(), 1},
		{"H", gate.H.Matrix(), 3},
		{"X", gate.X.Matrix(), 3}, // beta=pi, outer RZs are pi (cancel but not detected per-gate)
		{"U3(1,0.5,0.3)", gate.U3(1.0, 0.5, 0.3).Matrix(), 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := eulerZSX(tc.m, 0)
			if len(ops) > tc.maxOps {
				names := make([]string, len(ops))
				for i, op := range ops {
					names[i] = op.Gate.Name()
				}
				t.Errorf("eulerZSX(%s): got %d ops (max %d): %v", tc.name, len(ops), tc.maxOps, names)
			}
		})
	}
}

func TestEulerZSX_MaxOps(t *testing.T) {
	gates := []gate.Gate{
		gate.H, gate.X, gate.Y, gate.Z, gate.S, gate.Sdg, gate.T, gate.Tdg, gate.SX,
		gate.RX(math.Pi / 3), gate.RY(math.Pi / 4), gate.RZ(math.Pi / 6),
		gate.U3(1.0, 0.5, 0.3), gate.U3(0.7, 1.2, 2.3),
	}
	for _, g := range gates {
		ops := eulerZSX(g.Matrix(), 0)
		if len(ops) > 5 {
			t.Errorf("eulerZSX(%s): produced %d ops, expected at most 5", g.Name(), len(ops))
		}
	}
}

func TestEulerZSX_OnlyBasisGates(t *testing.T) {
	allowed := map[string]bool{"RZ": true, "SX": true, "X": true}
	gates := []gate.Gate{
		gate.H, gate.X, gate.Y, gate.Z, gate.S, gate.Sdg, gate.T, gate.Tdg, gate.SX,
		gate.RX(math.Pi / 3), gate.RY(math.Pi / 4), gate.RZ(math.Pi / 6),
		gate.U3(1.0, 0.5, 0.3),
	}
	for _, g := range gates {
		ops := eulerZSX(g.Matrix(), 0)
		for i, op := range ops {
			name := op.Gate.Name()
			// Strip parenthesized params.
			if idx := len(name); idx > 2 {
				base := name[:2]
				if base == "RZ" {
					continue
				}
			}
			if !allowed[name] {
				t.Errorf("eulerZSX(%s): op[%d] = %s not in {RZ, SX, X}", g.Name(), i, name)
			}
		}
	}
}

func TestBasisForTarget(t *testing.T) {
	cases := []struct {
		name     string
		basis    []string
		expected EulerBasis
	}{
		{"IBM", []string{"CX", "RZ", "SX", "X"}, BasisZSX},
		{"IBMwithI", []string{"CX", "RZ", "SX", "X", "I"}, BasisZSX},
		{"Quantinuum", []string{"RZZ", "RZ", "RY"}, BasisZYZ},
		{"Simulator", []string{"*"}, BasisZYZ},
		{"CustomZXZ", []string{"CX", "RX", "RZ"}, BasisZXZ},
		{"Empty", []string{}, BasisZYZ},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BasisForTarget(tc.basis)
			if got != tc.expected {
				t.Errorf("BasisForTarget(%v) = %d, want %d", tc.basis, got, tc.expected)
			}
		})
	}
}

func TestEulerDecomposeForBasis_AllBasesEquivalent(t *testing.T) {
	// All conventions should produce unitarily equivalent results.
	gates := []struct {
		name string
		gate gate.Gate
	}{
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"T", gate.T},
		{"SX", gate.SX},
		{"RX(pi/3)", gate.RX(math.Pi / 3)},
		{"RY(pi/4)", gate.RY(math.Pi / 4)},
		{"RZ(pi/6)", gate.RZ(math.Pi / 6)},
		{"U3(1,0.5,0.3)", gate.U3(1.0, 0.5, 0.3)},
	}
	bases := []EulerBasis{BasisZYZ, BasisZXZ, BasisZSX}

	for _, tc := range gates {
		t.Run(tc.name, func(t *testing.T) {
			for _, b := range bases {
				ops := EulerDecomposeForBasis(tc.gate, 0, b)
				if len(ops) == 0 {
					t.Fatalf("basis %d: no ops for %s", b, tc.name)
				}
				product := opsProduct(ops)
				if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-7); !ok {
					t.Errorf("basis %d: product does not match %s up to global phase", b, tc.name)
				}
			}
		})
	}
}

func TestEulerDecomposeForBasis_RejectsMultiQubit(t *testing.T) {
	for _, b := range []EulerBasis{BasisZYZ, BasisZXZ, BasisZSX} {
		ops := EulerDecomposeForBasis(gate.CNOT, 0, b)
		if ops != nil {
			t.Errorf("basis %d: expected nil for multi-qubit gate, got %d ops", b, len(ops))
		}
	}
}

func TestEulerZSX_Identity(t *testing.T) {
	ops := eulerZSX(gate.I.Matrix(), 0)
	if ops != nil {
		t.Errorf("eulerZSX(I): expected nil, got %d ops", len(ops))
	}
}

func TestEulerZXZ_Decompose_FixedGates(t *testing.T) {
	fixed := []struct {
		name string
		gate gate.Gate
	}{
		{"H", gate.H},
		{"X", gate.X},
		{"Y", gate.Y},
		{"Z", gate.Z},
		{"S", gate.S},
		{"SX", gate.SX},
	}
	for _, tc := range fixed {
		t.Run(tc.name, func(t *testing.T) {
			ops := eulerZXZ(tc.gate.Matrix(), 0)
			if len(ops) == 0 {
				t.Fatalf("eulerZXZ(%s): returned no operations", tc.name)
			}
			// Verify all ops are RZ or RX.
			for i, op := range ops {
				name := op.Gate.Name()
				if len(name) >= 2 && (name[:2] == "RZ" || name[:2] == "RX") {
					continue
				}
				t.Errorf("eulerZXZ(%s): op[%d] = %s, expected RZ or RX", tc.name, i, name)
			}
			product := opsProduct(ops)
			if _, ok := GlobalPhase(product, tc.gate.Matrix(), 1e-7); !ok {
				t.Errorf("eulerZXZ(%s): product does not match original up to global phase", tc.name)
			}
		})
	}
}
