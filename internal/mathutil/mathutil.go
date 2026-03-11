// Package mathutil provides shared math helpers for internal use.
package mathutil

import "math"

// NormalizeAngle wraps angle to (-pi, pi].
func NormalizeAngle(angle float64) float64 {
	a := math.Mod(angle, 2*math.Pi)
	if a > math.Pi {
		a -= 2 * math.Pi
	} else if a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// NearZeroMod2Pi reports whether angle is approximately 0 mod 2pi.
func NearZeroMod2Pi(angle float64) bool {
	a := math.Mod(angle, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a < 1e-10 || (2*math.Pi-a) < 1e-10
}

// StripParams strips parenthetical parameters from a gate name:
// "RZ(1.57)" -> "RZ", "H" -> "H".
func StripParams(name string) string {
	for i := range len(name) {
		if name[i] == '(' {
			return name[:i]
		}
	}
	return name
}

// StripParamsAndDagger strips both parenthetical parameters and a trailing
// dagger suffix ("†") from a gate name: "S†" -> "S", "RZ(1.57)" -> "RZ".
func StripParamsAndDagger(name string) string {
	// Strip "†" suffix (multi-byte UTF-8).
	for i := range len(name) {
		if name[i] >= 0x80 {
			name = name[:i]
			break
		}
	}
	// Strip parenthetical parameters.
	for i := range len(name) {
		if name[i] == '(' {
			name = name[:i]
			break
		}
	}
	return name
}
