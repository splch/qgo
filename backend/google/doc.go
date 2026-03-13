// Package google implements a [backend.Backend] for Google Quantum Engine
// via the REST-transcoded Quantum Engine API (quantum.googleapis.com).
//
// [New] creates a backend from a Google Cloud project ID. Authentication
// uses Application Default Credentials (ADC) via golang.org/x/oauth2/google.
// Use [WithCredentialsJSON] or [WithTokenSource] for explicit credentials.
//
// Circuits are serialized to Cirq JSON format using Google's native gate set
// (PhasedXZ + CZ). The API follows a two-step submission model: a Program
// (the circuit) is created first, then a Job is created under it.
//
// Options: [WithProcessor], [WithBaseURL], [WithCredentialsJSON],
// [WithTokenSource], [WithHTTPClient], [WithLogger].
package google
