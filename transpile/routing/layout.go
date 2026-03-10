// Package routing implements qubit routing algorithms for constrained connectivity.
package routing

// TrivialLayout returns the identity mapping: logical qubit i → physical qubit i.
func TrivialLayout(n int) []int {
	layout := make([]int, n)
	for i := range n {
		layout[i] = i
	}
	return layout
}

// InverseLayout returns the inverse of a layout mapping.
// If layout[logical] = physical, then inverse[physical] = logical.
func InverseLayout(layout []int) []int {
	inv := make([]int, len(layout))
	for log, phys := range layout {
		if phys >= 0 && phys < len(inv) {
			inv[phys] = log
		}
	}
	return inv
}
