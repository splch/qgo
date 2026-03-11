package param

import "testing"

func TestParameterNew(t *testing.T) {
	p := New("theta")
	if p.Name() != "theta" {
		t.Errorf("Name() = %q, want %q", p.Name(), "theta")
	}
}

func TestVector(t *testing.T) {
	v := NewVector("weights", 3)
	if v.Name() != "weights" {
		t.Errorf("Name() = %q, want %q", v.Name(), "weights")
	}
	if v.Size() != 3 {
		t.Errorf("Size() = %d, want 3", v.Size())
	}
	p0 := v.At(0)
	if p0.Name() != "weights[0]" {
		t.Errorf("At(0).Name() = %q, want %q", p0.Name(), "weights[0]")
	}
	p2 := v.At(2)
	if p2.Name() != "weights[2]" {
		t.Errorf("At(2).Name() = %q, want %q", p2.Name(), "weights[2]")
	}
}

func TestVectorPanics(t *testing.T) {
	v := NewVector("w", 2)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for out of range index")
		}
	}()
	v.At(5)
}
