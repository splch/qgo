// Package draw renders quantum circuits as text diagrams.
//
// The output uses a Cirq-style compact format with one row per qubit
// and columns for each gate moment:
//
//	q0: ---H---@---
//	           |
//	q1: -------X---
package draw
