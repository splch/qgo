package decompose

import (
	"math"
	"math/cmplx"
	"math/rand/v2"
	"testing"

	"github.com/splch/qgo/circuit/gate"
)

// randomHaarUnitary4 generates a Haar-random 4×4 unitary via QR decomposition
// of a random complex Gaussian matrix (Gram-Schmidt orthogonalization).
func randomHaarUnitary4(rng *rand.Rand) []complex128 {
	n := 4
	a := make([]complex128, n*n)
	for i := range a {
		a[i] = complex(rng.NormFloat64(), rng.NormFloat64())
	}
	q := make([]complex128, n*n)
	copy(q, a)
	for j := range n {
		for k := range j {
			dot := complex(0, 0)
			for i := range n {
				dot += cmplx.Conj(q[i*n+k]) * q[i*n+j]
			}
			for i := range n {
				q[i*n+j] -= dot * q[i*n+k]
			}
		}
		norm := 0.0
		for i := range n {
			norm += real(q[i*n+j] * cmplx.Conj(q[i*n+j]))
		}
		norm = math.Sqrt(norm)
		if norm < 1e-15 {
			norm = 1
		}
		for i := range n {
			q[i*n+j] /= complex(norm, 0)
		}
	}
	return q
}

func TestKAK_RandomUnitaries(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	const numTests = 200
	const tol = 1e-5

	for i := range numTests {
		m := randomHaarUnitary4(rng)
		ops := KAK(m, 0, 1)

		cx := countCNOTs(ops)
		if cx > 3 {
			t.Errorf("random[%d]: used %d CNOTs, max should be 3", i, cx)
		}

		if len(ops) > 0 {
			got := kakCircuitUnitary(ops, 0, 1)
			if _, ok := GlobalPhase(got, m, tol); !ok {
				t.Errorf("random[%d]: reconstructed unitary does not match original (up to global phase)", i)
			}
		}
	}
}

func TestKAK_ThreeCNOT_KnownGates(t *testing.T) {
	iswap := []complex128{1, 0, 0, 0, 0, 0, 1i, 0, 0, 1i, 0, 0, 0, 0, 0, 1}
	sqrtswap := []complex128{
		1, 0, 0, 0,
		0, complex(0.5, 0.5), complex(0.5, -0.5), 0,
		0, complex(0.5, -0.5), complex(0.5, 0.5), 0,
		0, 0, 0, 1,
	}

	cases := []struct {
		name    string
		mat     []complex128
		maxCNOT int
	}{
		{"iSWAP", iswap, 3},
		{"sqrt(SWAP)", sqrtswap, 3},
		{"CP(pi/7)", gate.CP(math.Pi / 7).Matrix(), 3},
		{"CRZ(pi/5)", gate.CRZ(math.Pi / 5).Matrix(), 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := KAK(tc.mat, 0, 1)
			if ops == nil {
				t.Fatal("returned nil")
			}
			cx := countCNOTs(ops)
			if cx > tc.maxCNOT {
				t.Errorf("expected at most %d CNOTs, got %d", tc.maxCNOT, cx)
			}
			got := kakCircuitUnitary(ops, 0, 1)
			if _, ok := GlobalPhase(got, tc.mat, 1e-6); !ok {
				t.Error("reconstructed unitary does not match up to global phase")
			}
		})
	}
}
