// Package target defines hardware target descriptions for transpilation.
package target

// Target describes a quantum hardware target.
type Target struct {
	Name            string
	NumQubits       int
	BasisGates      []string    // e.g., ["CX","RZ","SX","X"] or ["GPI","GPI2","MS"]
	Connectivity    []QubitPair // nil = all-to-all
	GateFidelities  map[string]float64
	MaxCircuitDepth int // 0 = unlimited
}

// QubitPair represents a connected pair of physical qubits.
type QubitPair struct{ Q0, Q1 int }

// HasBasisGate reports whether name is in the target's basis set.
// A basis set containing "*" matches all gates.
func (t Target) HasBasisGate(name string) bool {
	for _, b := range t.BasisGates {
		if b == "*" || b == name {
			return true
		}
	}
	return false
}

// HasDirection reports whether the target supports a 2Q gate from q0 to q1
// in that specific direction. Returns true for all-to-all targets (nil Connectivity).
// For directed targets, checks exact (Q0==q0, Q1==q1) match.
func (t Target) HasDirection(q0, q1 int) bool {
	if t.Connectivity == nil {
		return true
	}
	for _, p := range t.Connectivity {
		if p.Q0 == q0 && p.Q1 == q1 {
			return true
		}
	}
	return false
}

// IsConnected reports whether q0 and q1 are directly connected.
// Returns true for all-to-all targets (nil Connectivity).
func (t Target) IsConnected(q0, q1 int) bool {
	if t.Connectivity == nil {
		return true
	}
	for _, p := range t.Connectivity {
		if (p.Q0 == q0 && p.Q1 == q1) || (p.Q0 == q1 && p.Q1 == q0) {
			return true
		}
	}
	return false
}

// AdjacencyMap returns a map from qubit to its connected neighbors.
// Returns nil for all-to-all targets.
func (t Target) AdjacencyMap() map[int][]int {
	if t.Connectivity == nil {
		return nil
	}
	adj := make(map[int][]int)
	for _, p := range t.Connectivity {
		adj[p.Q0] = append(adj[p.Q0], p.Q1)
		adj[p.Q1] = append(adj[p.Q1], p.Q0)
	}
	return adj
}

// DistanceMatrix returns shortest-path distances between all qubit pairs
// using BFS. Returns nil for all-to-all targets.
func (t Target) DistanceMatrix() [][]int {
	if t.Connectivity == nil {
		return nil
	}
	adj := t.AdjacencyMap()
	n := t.NumQubits
	dist := make([][]int, n)
	for i := range n {
		dist[i] = make([]int, n)
		for j := range n {
			dist[i][j] = -1
		}
		// BFS from qubit i.
		dist[i][i] = 0
		queue := []int{i}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range adj[cur] {
				if dist[i][nb] == -1 {
					dist[i][nb] = dist[i][cur] + 1
					queue = append(queue, nb)
				}
			}
		}
	}
	return dist
}

// Predefined targets.
var (
	IonQForte = Target{
		Name:       "IonQ Forte",
		NumQubits:  36,
		BasisGates: []string{"GPI", "GPI2", "MS"},
		// all-to-all connectivity (nil)
	}

	IonQAria = Target{
		Name:       "IonQ Aria",
		NumQubits:  25,
		BasisGates: []string{"GPI", "GPI2", "MS"},
		// all-to-all connectivity (nil)
	}

	IBMEagle = Target{
		Name:       "IBM Eagle",
		NumQubits:  127,
		BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
		Connectivity: func() []QubitPair {
			// Heavy-hex topology for 127-qubit Eagle processor.
			// Simplified: linear chains with cross-links every 4th qubit.
			var pairs []QubitPair
			for i := range 126 {
				if i%2 == 0 { // backbone chain
					pairs = append(pairs, QubitPair{i, i + 1})
				}
			}
			// Cross-links for heavy-hex pattern.
			for i := 0; i < 120; i += 8 {
				pairs = append(pairs, QubitPair{i, i + 7})
				if i+4 < 127 {
					pairs = append(pairs, QubitPair{i + 3, i + 4})
				}
			}
			return pairs
		}(),
		MaxCircuitDepth: 0,
	}

	IBMBrisbane = Target{
		Name:       "ibm.brisbane",
		NumQubits:  127,
		BasisGates: []string{"CX", "RZ", "SX", "X", "I"},
		// Heavy-hex topology; connectivity fetched from calibration API.
	}

	IBMSherbrooke = Target{
		Name:       "ibm.sherbrooke",
		NumQubits:  127,
		BasisGates: []string{"CX", "RZ", "SX", "X", "I"},
		// Heavy-hex topology; connectivity fetched from calibration API.
	}

	QuantinuumH1 = Target{
		Name:       "Quantinuum H1",
		NumQubits:  20,
		BasisGates: []string{"RZZ", "RZ", "RY"},
		// all-to-all connectivity (nil)
	}

	QuantinuumH2 = Target{
		Name:       "Quantinuum H2",
		NumQubits:  56,
		BasisGates: []string{"RZZ", "RZ", "RY"},
		// all-to-all connectivity (nil)
	}

	// Quantinuum is a backward-compatible alias for QuantinuumH1.
	Quantinuum = QuantinuumH1

	GoogleWillow = Target{
		Name:       "Google Willow",
		NumQubits:  105,
		BasisGates: []string{"CZ", "RZ", "RX"},
		// 2D grid connectivity; nil = all-to-all approximation.
		// Exact connectivity can be fetched from the Quantum Engine API at runtime.
	}

	GoogleSycamore = Target{
		Name:       "Google Sycamore",
		NumQubits:  53,
		BasisGates: []string{"CZ", "RZ", "RX"},
		// 2D grid connectivity; nil = all-to-all approximation.
		// Exact connectivity can be fetched from the Quantum Engine API at runtime.
	}

	RigettiAnkaa = Target{
		Name:       "Rigetti Ankaa-3",
		NumQubits:  84,
		BasisGates: []string{"CZ", "RX", "RZ"},
		// Ankaa uses a square-octagon lattice; nil = all-to-all approximation.
		// Exact connectivity can be fetched from QCS ISA API at runtime.
	}

	Simulator = Target{
		Name:       "Simulator",
		NumQubits:  28,
		BasisGates: []string{"*"},
	}
)
