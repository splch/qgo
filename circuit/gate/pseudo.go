package gate

// Reset is a pseudo-gate that resets a qubit to |0⟩.
// It has no matrix representation — simulators handle it directly.
var Reset Gate = resetGate{}

type resetGate struct{}

func (g resetGate) Name() string                { return "reset" }
func (g resetGate) Qubits() int                 { return 1 }
func (g resetGate) Matrix() []complex128        { return nil }
func (g resetGate) Params() []float64           { return nil }
func (g resetGate) Inverse() Gate               { return g }
func (g resetGate) Decompose(_ []int) []Applied { return nil }
