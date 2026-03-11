package mathutil

import (
	"math"
	"testing"
)

func TestNormalizeAngle(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{0, 0},
		{math.Pi, math.Pi},
		{-math.Pi, math.Pi},    // edge: -pi wraps to pi
		{3 * math.Pi, math.Pi}, // 3pi wraps to pi
		{math.Pi / 2, math.Pi / 2},
		{-math.Pi / 2, -math.Pi / 2},
		{5 * math.Pi / 4, -3 * math.Pi / 4},
	}
	for _, tt := range tests {
		got := NormalizeAngle(tt.in)
		if math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("NormalizeAngle(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestNearZeroMod2Pi(t *testing.T) {
	tests := []struct {
		in   float64
		want bool
	}{
		{0, true},
		{2 * math.Pi, true},
		{-2 * math.Pi, true},
		{4 * math.Pi, true},
		{1e-11, true},
		{math.Pi, false},
		{1, false},
	}
	for _, tt := range tests {
		got := NearZeroMod2Pi(tt.in)
		if got != tt.want {
			t.Errorf("NearZeroMod2Pi(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestStripParams(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"H", "H"},
		{"RZ(1.57)", "RZ"},
		{"U3(0.1,0.2,0.3)", "U3"},
		{"CNOT", "CNOT"},
	}
	for _, tt := range tests {
		got := StripParams(tt.in)
		if got != tt.want {
			t.Errorf("StripParams(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripParamsAndDagger(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"H", "H"},
		{"S†", "S"},
		{"RZ(1.57)", "RZ"},
		{"RZ(1.57)†", "RZ"},
		{"CNOT", "CNOT"},
	}
	for _, tt := range tests {
		got := StripParamsAndDagger(tt.in)
		if got != tt.want {
			t.Errorf("StripParamsAndDagger(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
