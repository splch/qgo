// Package densitymatrix implements a density matrix quantum simulator
// supporting mixed states and noise channels.
//
// The simulator represents the quantum state as a density matrix ρ (rho),
// a dim×dim Hermitian positive-semidefinite matrix with Tr(ρ) = 1, where
// dim = 2^n for n qubits.
//
// Gate application uses the two-pass algorithm ρ' = U·ρ·U†. Noise channels
// use the Kraus representation ρ' = Σ_k E_k·ρ·E_k†.
//
// Maximum practical size is ~14 qubits (4 GB for the density matrix).
package densitymatrix
