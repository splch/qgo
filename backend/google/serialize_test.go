package google

import (
	"math"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestSerializeBellCircuit(t *testing.T) {
	// Bell circuit: H(0) + CNOT(0,1) should decompose to:
	// Moment 0: H(0), H(1)   [H(0) from original, H(1) from CNOT decomp]
	// Moment 1: CZ(0,1)      [from CNOT decomp]
	// Moment 2: H(1)          [from CNOT decomp]
	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	program, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if program.Type != "Circuit" {
		t.Errorf("program type = %q, want Circuit", program.Type)
	}
	if len(program.Qubits) != 2 {
		t.Errorf("qubits = %d, want 2", len(program.Qubits))
	}

	// Verify CZ gate appears somewhere in the moments.
	foundCZ := false
	for _, m := range program.Moments {
		for _, op := range m.Operations {
			if op.Gate.Type == "CZPowGate" {
				foundCZ = true
				if op.Gate.Exponent != 1.0 {
					t.Errorf("CZ exponent = %f, want 1.0", op.Gate.Exponent)
				}
			}
		}
	}
	if !foundCZ {
		t.Error("expected CZ gate in decomposed Bell circuit")
	}

	// Verify PhasedXZGate appears (from H gates).
	foundPXZ := false
	for _, m := range program.Moments {
		for _, op := range m.Operations {
			if op.Gate.Type == "PhasedXZGate" {
				foundPXZ = true
			}
		}
	}
	if !foundPXZ {
		t.Error("expected PhasedXZGate in decomposed Bell circuit")
	}
}

func TestSerializeFixedGates(t *testing.T) {
	tests := []struct {
		name      string
		buildFunc func(b *builder.Builder) *builder.Builder
		wantX     float64
		wantZ     float64
		wantA     float64
	}{
		{"H", func(b *builder.Builder) *builder.Builder { return b.H(0) }, 1.0, 1.0, 0.0},
		{"X", func(b *builder.Builder) *builder.Builder { return b.X(0) }, 1.0, 0.0, 0.0},
		{"Y", func(b *builder.Builder) *builder.Builder { return b.Y(0) }, 1.0, 0.0, 0.5},
		{"Z", func(b *builder.Builder) *builder.Builder { return b.Z(0) }, 0.0, 1.0, 0.0},
		{"S", func(b *builder.Builder) *builder.Builder { return b.S(0) }, 0.0, 0.5, 0.0},
		{"T", func(b *builder.Builder) *builder.Builder { return b.T(0) }, 0.0, 0.25, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := builder.New("test", 1)
			c, _ := tt.buildFunc(b).Build()
			prog, err := serializeCircuit(c)
			if err != nil {
				t.Fatal(err)
			}

			if len(prog.Moments) == 0 {
				t.Fatal("expected at least one moment")
			}

			op := prog.Moments[0].Operations[0]
			if op.Gate.Type != "PhasedXZGate" {
				t.Fatalf("gate type = %q, want PhasedXZGate", op.Gate.Type)
			}
			if op.Gate.Exponent != tt.wantX {
				t.Errorf("x_exponent = %f, want %f", op.Gate.Exponent, tt.wantX)
			}
			if op.Gate.PhaseExp != tt.wantZ {
				t.Errorf("z_exponent = %f, want %f", op.Gate.PhaseExp, tt.wantZ)
			}
			if op.Gate.AxisPhaseExp != tt.wantA {
				t.Errorf("axis_phase_exp = %f, want %f", op.Gate.AxisPhaseExp, tt.wantA)
			}
		})
	}
}

func TestSerializeRotationGates(t *testing.T) {
	tests := []struct {
		name  string
		gate  gate.Gate
		wantX float64
		wantZ float64
		wantA float64
	}{
		{"RX(pi)", gate.RX(math.Pi), 1.0, 0.0, 0.0},
		{"RX(pi/2)", gate.RX(math.Pi / 2), 0.5, 0.0, 0.0},
		{"RY(pi)", gate.RY(math.Pi), 1.0, 0.0, 0.25},
		{"RZ(pi)", gate.RZ(math.Pi), 0.0, 1.0, 0.0},
		{"RZ(pi/4)", gate.RZ(math.Pi / 4), 0.0, 0.25, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := []ir.Operation{{Gate: tt.gate, Qubits: []int{0}}}
			c := ir.New("test", 1, 0, ops, nil)
			prog, err := serializeCircuit(c)
			if err != nil {
				t.Fatal(err)
			}

			if len(prog.Moments) == 0 {
				t.Fatal("expected at least one moment")
			}

			op := prog.Moments[0].Operations[0]
			if op.Gate.Type != "PhasedXZGate" {
				t.Fatalf("gate type = %q, want PhasedXZGate", op.Gate.Type)
			}
			const tol = 1e-10
			if math.Abs(op.Gate.Exponent-tt.wantX) > tol {
				t.Errorf("x_exponent = %f, want %f", op.Gate.Exponent, tt.wantX)
			}
			if math.Abs(op.Gate.PhaseExp-tt.wantZ) > tol {
				t.Errorf("z_exponent = %f, want %f", op.Gate.PhaseExp, tt.wantZ)
			}
			if math.Abs(op.Gate.AxisPhaseExp-tt.wantA) > tol {
				t.Errorf("axis_phase_exp = %f, want %f", op.Gate.AxisPhaseExp, tt.wantA)
			}
		})
	}
}

func TestSerializeCZ(t *testing.T) {
	ops := []ir.Operation{{Gate: gate.CZ, Qubits: []int{0, 1}}}
	c := ir.New("test", 2, 0, ops, nil)
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if len(prog.Moments) != 1 {
		t.Fatalf("moments = %d, want 1", len(prog.Moments))
	}
	op := prog.Moments[0].Operations[0]
	if op.Gate.Type != "CZPowGate" {
		t.Errorf("gate type = %q, want CZPowGate", op.Gate.Type)
	}
	if op.Gate.Exponent != 1.0 {
		t.Errorf("exponent = %f, want 1.0", op.Gate.Exponent)
	}
	if len(op.Qubits) != 2 {
		t.Errorf("qubits = %d, want 2", len(op.Qubits))
	}
}

func TestSerializeSWAP(t *testing.T) {
	ops := []ir.Operation{{Gate: gate.SWAP, Qubits: []int{0, 1}}}
	c := ir.New("test", 2, 0, ops, nil)
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	// SWAP decomposes to 3 CNOTs, each to H+CZ+H = 9 native ops.
	// Count CZ gates.
	czCount := 0
	for _, m := range prog.Moments {
		for _, op := range m.Operations {
			if op.Gate.Type == "CZPowGate" {
				czCount++
			}
		}
	}
	if czCount != 3 {
		t.Errorf("CZ count = %d, want 3", czCount)
	}
}

func TestSerializeMeasurement(t *testing.T) {
	c, _ := builder.New("test", 2).WithClbits(2).H(0).CNOT(0, 1).Measure(0, 0).Measure(1, 1).Build()
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	measCount := 0
	for _, m := range prog.Moments {
		for _, op := range m.Operations {
			if op.Gate.Type == "MeasurementGate" {
				measCount++
			}
		}
	}
	if measCount != 2 {
		t.Errorf("measurement count = %d, want 2", measCount)
	}
}

func TestSerializeIdentitySkipped(t *testing.T) {
	ops := []ir.Operation{{Gate: gate.I, Qubits: []int{0}}}
	c := ir.New("test", 1, 0, ops, nil)
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	// Identity should be skipped entirely.
	totalOps := 0
	for _, m := range prog.Moments {
		totalOps += len(m.Operations)
	}
	if totalOps != 0 {
		t.Errorf("total ops = %d, want 0 (identity should be skipped)", totalOps)
	}
}

func TestSerializeUnsupportedGate(t *testing.T) {
	ops := []ir.Operation{{Gate: gate.CCX, Qubits: []int{0, 1, 2}}}
	c := ir.New("test", 3, 0, ops, nil)
	_, err := serializeCircuit(c)
	if err == nil {
		t.Fatal("expected error for unsupported CCX gate")
	}
}

func TestMomentScheduling(t *testing.T) {
	// Two independent gates on different qubits should be in the same moment.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	c := ir.New("test", 2, 0, ops, nil)
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if len(prog.Moments) != 1 {
		t.Errorf("moments = %d, want 1 (parallel single-qubit gates)", len(prog.Moments))
	}
	if len(prog.Moments[0].Operations) != 2 {
		t.Errorf("ops in moment 0 = %d, want 2", len(prog.Moments[0].Operations))
	}
}

func TestMomentSchedulingConflict(t *testing.T) {
	// Two gates on the same qubit must be in different moments.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
	}
	c := ir.New("test", 1, 0, ops, nil)
	prog, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}

	if len(prog.Moments) != 2 {
		t.Errorf("moments = %d, want 2 (sequential single-qubit gates)", len(prog.Moments))
	}
}

func TestSampleToBitstring(t *testing.T) {
	tests := []struct {
		sample []int
		want   string
	}{
		{[]int{0, 0}, "00"},
		{[]int{1, 1}, "11"},
		{[]int{1, 0, 1}, "101"},
		{[]int{0}, "0"},
	}
	for _, tt := range tests {
		got := sampleToBitstring(tt.sample)
		if got != tt.want {
			t.Errorf("sampleToBitstring(%v) = %q, want %q", tt.sample, got, tt.want)
		}
	}
}

func TestParseResultsEmpty(t *testing.T) {
	cr := cirqResult{}
	result, err := parseResults(cr, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shots != 100 {
		t.Errorf("shots = %d, want 100", result.Shots)
	}
	if len(result.Counts) != 0 {
		t.Errorf("counts = %d, want 0", len(result.Counts))
	}
}

func TestParseResultsWithData(t *testing.T) {
	cr := cirqResult{
		MeasurementResults: []measurementResult{{
			Key:         "m",
			Repetitions: 4,
			Results: [][]int{
				{0, 0},
				{0, 0},
				{1, 1},
				{1, 1},
			},
		}},
	}
	result, err := parseResults(cr, 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts["00"] != 2 {
		t.Errorf("Counts[00] = %d, want 2", result.Counts["00"])
	}
	if result.Counts["11"] != 2 {
		t.Errorf("Counts[11] = %d, want 2", result.Counts["11"])
	}
	if result.Shots != 4 {
		t.Errorf("Shots = %d, want 4", result.Shots)
	}
}
