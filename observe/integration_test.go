package observe_test

import (
	"context"
	"sync"
	"testing"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/backend/local"
	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/job/manager"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/transpile/pipeline"
	"github.com/splch/qgo/transpile/target"
)

func bellCircuit(t *testing.T) *ir.Circuit {
	t.Helper()
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).MeasureAll().Build()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHooksFireDuringManagerSubmit(t *testing.T) {
	var mu sync.Mutex
	var events []string

	h := &observe.Hooks{
		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			mu.Lock()
			events = append(events, "job_start")
			mu.Unlock()
			return ctx, func(jobID string, err error) {
				mu.Lock()
				events = append(events, "job_done")
				mu.Unlock()
			}
		},
		WrapSim: func(ctx context.Context, info observe.SimInfo) (context.Context, func(error)) {
			mu.Lock()
			events = append(events, "sim_start")
			mu.Unlock()
			if info.NumQubits != 2 {
				t.Errorf("sim qubits = %d, want 2", info.NumQubits)
			}
			if info.Shots != 100 {
				t.Errorf("sim shots = %d, want 100", info.Shots)
			}
			return ctx, func(err error) {
				mu.Lock()
				events = append(events, "sim_done")
				mu.Unlock()
			}
		},
		OnJobPoll: func(ctx context.Context, info observe.JobPollInfo) {
			mu.Lock()
			events = append(events, "poll")
			mu.Unlock()
		},
	}

	ctx := observe.WithHooks(context.Background(), h)

	m := manager.New()
	m.Register("local", local.New())

	c := bellCircuit(t)
	result, err := m.Submit(ctx, "local", &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	mu.Lock()
	defer mu.Unlock()

	// Expect: job_start, sim_start, sim_done, poll, job_done
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d: %v", len(events), events)
	}
	if events[0] != "job_start" {
		t.Errorf("first event = %q, want %q", events[0], "job_start")
	}
	// sim_start and sim_done should both appear
	hasSim := false
	for _, e := range events {
		if e == "sim_start" {
			hasSim = true
			break
		}
	}
	if !hasSim {
		t.Errorf("missing sim_start in events: %v", events)
	}
	// job_done should be last
	if events[len(events)-1] != "job_done" {
		t.Errorf("last event = %q, want %q", events[len(events)-1], "job_done")
	}
}

func TestHooksFireDuringTranspile(t *testing.T) {
	var transpileStarted, transpileDone bool
	var passNames []string
	var mu sync.Mutex

	h := &observe.Hooks{
		WrapTranspile: func(ctx context.Context, level int, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			transpileStarted = true
			if level != int(pipeline.LevelBasic) {
				t.Errorf("level = %d, want %d", level, pipeline.LevelBasic)
			}
			return ctx, func(out observe.CircuitInfo, err error) {
				transpileDone = true
			}
		},
		WrapPass: func(ctx context.Context, pass string, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			mu.Lock()
			passNames = append(passNames, pass)
			mu.Unlock()
			return ctx, func(out observe.CircuitInfo, err error) {}
		},
	}

	ctx := observe.WithHooks(context.Background(), h)

	c := bellCircuit(t)
	result, err := pipeline.Run(ctx, c, target.Simulator, pipeline.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	if !transpileStarted {
		t.Error("WrapTranspile never called")
	}
	if !transpileDone {
		t.Error("transpile done function never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(passNames) == 0 {
		t.Error("no passes observed")
	}

	// Should include known pass names
	hasDecompose := false
	hasValidate := false
	for _, n := range passNames {
		if n == "decompose_to_target" {
			hasDecompose = true
		}
		if n == "validate_target" {
			hasValidate = true
		}
	}
	if !hasDecompose {
		t.Errorf("missing decompose_to_target in passes: %v", passNames)
	}
	if !hasValidate {
		t.Errorf("missing validate_target in passes: %v", passNames)
	}
}

func TestNoHooksNoError(t *testing.T) {
	// Ensure everything works fine with no hooks in context.
	ctx := context.Background()

	m := manager.New()
	m.Register("local", local.New())

	c := bellCircuit(t)
	result, err := m.Submit(ctx, "local", &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("nil result")
	}

	// Transpile with no hooks
	_, err = pipeline.Run(ctx, c, target.Simulator, pipeline.LevelFull)
	if err != nil {
		t.Fatal(err)
	}
}
