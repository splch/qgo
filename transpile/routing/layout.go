// Package routing implements qubit routing algorithms for constrained connectivity.
package routing

import "math/rand/v2"

// TrivialLayout returns the identity mapping: logical qubit i → physical qubit i.
func TrivialLayout(n int) []int {
	layout := make([]int, n)
	for i := range n {
		layout[i] = i
	}
	return layout
}

// RandomLayout returns a random permutation of [0, n) using Fisher-Yates.
func RandomLayout(n int, rng *rand.Rand) []int {
	layout := TrivialLayout(n)
	for i := n - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		layout[i], layout[j] = layout[j], layout[i]
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

// copyLayout returns a copy of the layout slice.
func copyLayout(layout []int) []int {
	out := make([]int, len(layout))
	copy(out, layout)
	return out
}
