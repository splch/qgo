// Package noise defines quantum noise channels and noise models for
// use with the density matrix simulator.
//
// A [Channel] represents a quantum noise channel as a set of Kraus operators.
// A [NoiseModel] maps gate operations to noise channels, with a resolution
// order of qubit-specific > gate-name > qubit-count default.
package noise
