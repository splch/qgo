package observe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/splch/qgo/observe"
)

func TestWithHooksRoundTrip(t *testing.T) {
	h := &observe.Hooks{}
	ctx := observe.WithHooks(context.Background(), h)
	got := observe.FromContext(ctx)
	if got != h {
		t.Fatal("FromContext did not return the same Hooks")
	}
}

func TestFromContextNil(t *testing.T) {
	got := observe.FromContext(context.Background())
	if got != nil {
		t.Fatal("expected nil hooks from empty context")
	}
}

func TestHooksNilFieldsSafe(t *testing.T) {
	h := &observe.Hooks{}
	ctx := observe.WithHooks(context.Background(), h)
	got := observe.FromContext(ctx)

	// All fields should be nil on a zero-value Hooks.
	if got.WrapTranspile != nil {
		t.Error("expected nil WrapTranspile")
	}
	if got.WrapPass != nil {
		t.Error("expected nil WrapPass")
	}
	if got.WrapJob != nil {
		t.Error("expected nil WrapJob")
	}
	if got.WrapSim != nil {
		t.Error("expected nil WrapSim")
	}
	if got.WrapHTTP != nil {
		t.Error("expected nil WrapHTTP")
	}
	if got.OnJobPoll != nil {
		t.Error("expected nil OnJobPoll")
	}
}

func TestWrapTranspileCallback(t *testing.T) {
	var startLevel int
	var startIn observe.CircuitInfo
	var doneOut observe.CircuitInfo
	var doneErr error

	h := &observe.Hooks{
		WrapTranspile: func(ctx context.Context, level int, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			startLevel = level
			startIn = in
			return ctx, func(out observe.CircuitInfo, err error) {
				doneOut = out
				doneErr = err
			}
		},
	}

	ctx := observe.WithHooks(context.Background(), h)
	hooks := observe.FromContext(ctx)
	in := observe.CircuitInfo{NumQubits: 4, GateCount: 10, Depth: 5}
	ctx, done := hooks.WrapTranspile(ctx, 2, in)
	_ = ctx
	done(observe.CircuitInfo{NumQubits: 4, GateCount: 6, Depth: 3}, nil)

	if startLevel != 2 {
		t.Errorf("level = %d, want 2", startLevel)
	}
	if startIn.GateCount != 10 {
		t.Errorf("startIn.GateCount = %d, want 10", startIn.GateCount)
	}
	if doneOut.GateCount != 6 {
		t.Errorf("doneOut.GateCount = %d, want 6", doneOut.GateCount)
	}
	if doneErr != nil {
		t.Errorf("doneErr = %v, want nil", doneErr)
	}
}

func TestWrapJobCallback(t *testing.T) {
	var gotInfo observe.JobInfo
	var doneJobID string
	var doneErr error

	h := &observe.Hooks{
		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			gotInfo = info
			return ctx, func(jobID string, err error) {
				doneJobID = jobID
				doneErr = err
			}
		},
	}

	ctx := observe.WithHooks(context.Background(), h)
	hooks := observe.FromContext(ctx)

	info := observe.JobInfo{Backend: "ionq.simulator", Shots: 1000, Qubits: 4}
	ctx, done := hooks.WrapJob(ctx, info)
	_ = ctx
	done("job-123", nil)

	if gotInfo.Backend != "ionq.simulator" {
		t.Errorf("backend = %q, want %q", gotInfo.Backend, "ionq.simulator")
	}
	if doneJobID != "job-123" {
		t.Errorf("jobID = %q, want %q", doneJobID, "job-123")
	}
	if doneErr != nil {
		t.Errorf("err = %v, want nil", doneErr)
	}
}

func TestWrapJobCallbackWithError(t *testing.T) {
	wantErr := errors.New("submission failed")
	var doneErr error

	h := &observe.Hooks{
		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			return ctx, func(jobID string, err error) {
				doneErr = err
			}
		},
	}

	ctx := observe.WithHooks(context.Background(), h)
	hooks := observe.FromContext(ctx)
	ctx, done := hooks.WrapJob(ctx, observe.JobInfo{})
	_ = ctx
	done("", wantErr)

	if doneErr != wantErr {
		t.Errorf("err = %v, want %v", doneErr, wantErr)
	}
}

func TestOnJobPollCallback(t *testing.T) {
	var gotPoll observe.JobPollInfo

	h := &observe.Hooks{
		OnJobPoll: func(ctx context.Context, info observe.JobPollInfo) {
			gotPoll = info
		},
	}

	ctx := observe.WithHooks(context.Background(), h)
	hooks := observe.FromContext(ctx)

	poll := observe.JobPollInfo{
		Backend:  "local.simulator",
		JobID:    "job-456",
		State:    "running",
		Attempt:  3,
		QueuePos: -1,
	}
	hooks.OnJobPoll(ctx, poll)

	if gotPoll.JobID != "job-456" {
		t.Errorf("jobID = %q, want %q", gotPoll.JobID, "job-456")
	}
	if gotPoll.Attempt != 3 {
		t.Errorf("attempt = %d, want 3", gotPoll.Attempt)
	}
}
