package densitymatrix

import (
	"runtime"
	"sync"
)

// applyGate1 applies a 1-qubit gate: ρ' = U·ρ·U†.
// Two-pass: first multiply U on row indices, then U† on column indices.
func (s *Sim) applyGate1(qubit int, m []complex128) {
	if s.numQubits >= parallelThreshold {
		s.applyGate1Parallel(qubit, m)
		return
	}
	s.gate1Rows(qubit, m)
	s.gate1Cols(qubit, m)
}

// gate1Rows applies U on the row index: for pairs of rows (r0, r1) differing in bit q,
// apply U to rho[r0][c], rho[r1][c] for all columns c.
func (s *Sim) gate1Rows(qubit int, m []complex128) {
	dim := s.dim
	halfBlock := 1 << qubit
	block := halfBlock << 1

	for b0 := 0; b0 < dim; b0 += block {
		for offset := range halfBlock {
			r0 := b0 + offset
			r1 := r0 + halfBlock
			base0 := r0 * dim
			base1 := r1 * dim
			for c := range dim {
				a0 := s.rho[base0+c]
				a1 := s.rho[base1+c]
				s.rho[base0+c] = m[0]*a0 + m[1]*a1
				s.rho[base1+c] = m[2]*a0 + m[3]*a1
			}
		}
	}
}

// gate1Cols right-multiplies by U† on the column index.
// For pairs (c0, c1) differing in bit q: rho'[r][c] = Σ_l rho[r][l] * conj(U[c_q][l_q]).
// This is equivalent to applying conj(U) as a left-multiplication on column indices.
func (s *Sim) gate1Cols(qubit int, m []complex128) {
	dim := s.dim
	halfBlock := 1 << qubit
	block := halfBlock << 1

	// For right-multiply by U†: use conj(U) element-wise (not conjugate transpose).
	uc00, uc01 := conj(m[0]), conj(m[1])
	uc10, uc11 := conj(m[2]), conj(m[3])

	for r := range dim {
		base := r * dim
		for b0 := 0; b0 < dim; b0 += block {
			for offset := range halfBlock {
				c0 := b0 + offset
				c1 := c0 + halfBlock
				a0 := s.rho[base+c0]
				a1 := s.rho[base+c1]
				s.rho[base+c0] = uc00*a0 + uc01*a1
				s.rho[base+c1] = uc10*a0 + uc11*a1
			}
		}
	}
}

// applyGate1Parallel is the parallel version.
func (s *Sim) applyGate1Parallel(qubit int, m []complex128) {
	s.gate1RowsParallel(qubit, m)
	s.gate1ColsParallel(qubit, m)
}

func (s *Sim) gate1RowsParallel(qubit int, m []complex128) {
	dim := s.dim
	halfBlock := 1 << qubit
	block := halfBlock << 1
	nBlocks := dim / block

	nWorkers := optimalWorkers(s.numQubits)
	if nBlocks < nWorkers {
		nWorkers = nBlocks
	}
	if nWorkers < 1 {
		nWorkers = 1
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	blocksPerWorker := nBlocks / nWorkers
	for w := range nWorkers {
		startBlock := w * blocksPerWorker
		endBlock := startBlock + blocksPerWorker
		if w == nWorkers-1 {
			endBlock = nBlocks
		}
		go func(sb, eb int) {
			defer wg.Done()
			for b := sb; b < eb; b++ {
				b0 := b * block
				for offset := range halfBlock {
					r0 := b0 + offset
					r1 := r0 + halfBlock
					base0 := r0 * dim
					base1 := r1 * dim
					for c := range dim {
						a0 := s.rho[base0+c]
						a1 := s.rho[base1+c]
						s.rho[base0+c] = m[0]*a0 + m[1]*a1
						s.rho[base1+c] = m[2]*a0 + m[3]*a1
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) gate1ColsParallel(qubit int, m []complex128) {
	dim := s.dim
	halfBlock := 1 << qubit
	block := halfBlock << 1

	uc00, uc01 := conj(m[0]), conj(m[1])
	uc10, uc11 := conj(m[2]), conj(m[3])

	nWorkers := optimalWorkers(s.numQubits)
	if dim < nWorkers {
		nWorkers = dim
	}
	if nWorkers < 1 {
		nWorkers = 1
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	rowsPerWorker := dim / nWorkers
	for w := range nWorkers {
		startRow := w * rowsPerWorker
		endRow := startRow + rowsPerWorker
		if w == nWorkers-1 {
			endRow = dim
		}
		go func(sr, er int) {
			defer wg.Done()
			for r := sr; r < er; r++ {
				base := r * dim
				for b0 := 0; b0 < dim; b0 += block {
					for offset := range halfBlock {
						c0 := b0 + offset
						c1 := c0 + halfBlock
						a0 := s.rho[base+c0]
						a1 := s.rho[base+c1]
						s.rho[base+c0] = uc00*a0 + uc01*a1
						s.rho[base+c1] = uc10*a0 + uc11*a1
					}
				}
			}
		}(startRow, endRow)
	}
	wg.Wait()
}

func optimalWorkers(nQubits int) int {
	if nQubits < parallelThreshold {
		return 1
	}
	maxProcs := runtime.GOMAXPROCS(0)
	dim := 1 << nQubits
	maxByWork := (dim * dim) / 8192
	if maxByWork < 1 {
		maxByWork = 1
	}
	if maxProcs < maxByWork {
		return maxProcs
	}
	return maxByWork
}
