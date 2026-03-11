package noise

// ReadoutError represents classical measurement error.
// P01 is P(measure 1 | state 0), P10 is P(measure 0 | state 1).
type ReadoutError struct {
	P01 float64 // probability of reading 1 when state is 0
	P10 float64 // probability of reading 0 when state is 1
}

// NewReadoutError creates a ReadoutError with the given probabilities.
func NewReadoutError(p01, p10 float64) *ReadoutError {
	if p01 < 0 || p01 > 1 || p10 < 0 || p10 > 1 {
		panic("noise.NewReadoutError: probabilities must be in [0,1]")
	}
	return &ReadoutError{P01: p01, P10: p10}
}

// Apply applies readout error to ideal probabilities.
// p0 is the probability of measuring 0, p1 = 1-p0.
// Returns noisy (p0', p1').
func (r *ReadoutError) Apply(p0, p1 float64) (float64, float64) {
	np0 := (1-r.P01)*p0 + r.P10*p1
	np1 := r.P01*p0 + (1-r.P10)*p1
	return np0, np1
}
