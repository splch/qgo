// Package quantinuum implements a [backend.Backend] for Quantinuum's
// H-series trapped-ion quantum computers via the Quantinuum Machine API (v1).
//
// [New] creates a backend from email/password credentials. Use [WithDevice]
// to select a target (e.g. "H1-1", "H2-1", "H1-1E" for emulator, "H1-1SC"
// for syntax checker). Circuits are serialized as OpenQASM 2.0 for submission.
//
// Authentication uses JWT tokens from the /login endpoint, cached and
// refreshed automatically. HTTP requests use exponential backoff for 429
// and 5xx responses.
//
// Options: [WithBaseURL], [WithLoginURL], [WithHTTPClient], [WithLogger].
package quantinuum
