package densitymatrix

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/sim/noise"
)

// parallelThreshold is the minimum number of qubits before enabling parallel kernels.
// At 9 qubits dim=512 and the density matrix has 262K elements; the heavier
// per-element work (row + column passes) justifies a lower threshold than statevector.
const parallelThreshold = 9

// Sim simulates a circuit via density matrix evolution.
type Sim struct {
	numQubits int
	dim       int // 2^numQubits
	rho       []complex128
	noise     *noise.NoiseModel
}

// New creates a simulator initialized to |0...0⟩⟨0...0|.
func New(numQubits int) *Sim {
	if numQubits < 1 || numQubits > 14 {
		panic(fmt.Sprintf("densitymatrix: numQubits %d out of range [1, 14]", numQubits))
	}
	dim := 1 << numQubits
	rho := make([]complex128, dim*dim)
	rho[0] = 1 // |0><0|
	return &Sim{numQubits: numQubits, dim: dim, rho: rho}
}

// WithNoise sets the noise model for the simulation.
func (s *Sim) WithNoise(nm *noise.NoiseModel) *Sim {
	s.noise = nm
	return s
}

// Run executes the circuit and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	if err := s.Evolve(c); err != nil {
		return nil, err
	}
	probs := s.diagonalProbs()
	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for range shots {
		idx := sampleIndex(probs, rng)
		bs := formatBitstring(idx, s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

// Evolve applies all gate operations, resets state to |0><0| first.
func (s *Sim) Evolve(c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	// Reset to |0><0|.
	for i := range s.rho {
		s.rho[i] = 0
	}
	s.rho[0] = 1

	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		m := op.Gate.Matrix()
		if m == nil {
			continue
		}
		switch op.Gate.Qubits() {
		case 1:
			s.applyGate1(op.Qubits[0], m)
		case 2:
			s.applyGate2(op.Qubits[0], op.Qubits[1], m)
		default:
			return fmt.Errorf("densitymatrix: unsupported gate size: %d qubits", op.Gate.Qubits())
		}
		// Apply noise after gate.
		if s.noise != nil {
			ch := s.noise.Lookup(op.Gate.Name(), op.Qubits)
			if ch != nil {
				s.applyChannel(ch, op.Qubits)
			}
		}
	}
	return nil
}

// applyChannel applies a noise channel ρ' = Σ_k E_k ρ E_k†.
func (s *Sim) applyChannel(ch noise.Channel, qubits []int) {
	kraus := ch.Kraus()
	if len(kraus) == 1 {
		// Single Kraus op: in-place like a gate.
		switch ch.Qubits() {
		case 1:
			s.applyGate1(qubits[0], kraus[0])
		case 2:
			s.applyGate2(qubits[0], qubits[1], kraus[0])
		}
		return
	}

	// Multiple Kraus ops: accumulate into temp buffer.
	n := len(s.rho)
	temp := make([]complex128, n)
	saved := make([]complex128, n)
	copy(saved, s.rho)

	for _, ek := range kraus {
		// Restore rho from saved.
		copy(s.rho, saved)
		// Apply E_k as a gate.
		switch ch.Qubits() {
		case 1:
			s.applyGate1(qubits[0], ek)
		case 2:
			s.applyGate2(qubits[0], qubits[1], ek)
		}
		// Accumulate.
		for i := range n {
			temp[i] += s.rho[i]
		}
	}
	copy(s.rho, temp)
}

// DensityMatrix returns a copy of the current density matrix.
func (s *Sim) DensityMatrix() []complex128 {
	out := make([]complex128, len(s.rho))
	copy(out, s.rho)
	return out
}

// Purity returns Tr(ρ²). Pure states have purity 1, maximally mixed = 1/dim.
func (s *Sim) Purity() float64 {
	dim := s.dim
	var tr complex128
	// Tr(ρ²) = Σ_{ij} ρ_{ij} * ρ_{ji} = Σ_{ij} |ρ_{ij}|²  (since ρ is Hermitian)
	for i := range dim {
		for j := range dim {
			v := s.rho[i*dim+j]
			tr += v * conj(s.rho[j*dim+i])
		}
	}
	return math.Abs(real(tr))
}

// Fidelity computes the fidelity F = ⟨ψ|ρ|ψ⟩ between a pure state |ψ⟩ and ρ.
func (s *Sim) Fidelity(pureState []complex128) float64 {
	if len(pureState) != s.dim {
		panic(fmt.Sprintf("densitymatrix: pure state length %d, expected %d", len(pureState), s.dim))
	}
	dim := s.dim
	var f complex128
	for i := range dim {
		for j := range dim {
			f += conj(pureState[i]) * s.rho[i*dim+j] * pureState[j]
		}
	}
	return math.Abs(real(f))
}

func conj(c complex128) complex128 {
	return complex(real(c), -imag(c))
}
