package rigetti

import (
	"context"
	"fmt"
	"testing"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/backend/rigetti/internal/qcs"
	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/transpile/target"
)

// mockTranslation implements translationAPI for testing.
type mockTranslation struct {
	resp *qcs.TranslateResponse
	err  error
}

func (m *mockTranslation) TranslateQuilToEncryptedControllerJob(_ context.Context, _ *qcs.TranslateRequest) (*qcs.TranslateResponse, error) {
	return m.resp, m.err
}

// mockController implements controllerAPI for testing.
type mockController struct {
	execResp    *qcs.ExecuteResponse
	execErr     error
	statusResp  *qcs.StatusResponse
	statusErr   error
	resultsResp *qcs.ResultsResponse
	resultsErr  error
	cancelErr   error
	cancelCalls int
}

func (m *mockController) ExecuteControllerJob(_ context.Context, _ *qcs.ExecuteRequest) (*qcs.ExecuteResponse, error) {
	return m.execResp, m.execErr
}

func (m *mockController) GetControllerJobStatus(_ context.Context, _ *qcs.StatusRequest) (*qcs.StatusResponse, error) {
	return m.statusResp, m.statusErr
}

func (m *mockController) GetControllerJobResults(_ context.Context, _ *qcs.ResultsRequest) (*qcs.ResultsResponse, error) {
	return m.resultsResp, m.resultsErr
}

func (m *mockController) CancelControllerJobs(_ context.Context, _ *qcs.CancelRequest) (*qcs.CancelResponse, error) {
	m.cancelCalls++
	return &qcs.CancelResponse{}, m.cancelErr
}

// newTestBackend creates a Backend with mock services (no credentials needed).
func newTestBackend(t *testing.T, trans *mockTranslation, ctrl *mockController) *Backend {
	t.Helper()
	b, err := New(
		WithAccessToken("test-token"),
		WithProcessor("Ankaa-3"),
		withTranslation(trans),
		withController(ctrl),
	)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSubmitAndResult(t *testing.T) {
	trans := &mockTranslation{
		resp: &qcs.TranslateResponse{
			EncryptedProgram: []byte("encrypted-program"),
			ReadoutMap:       map[string]string{"0": "0", "1": "1"},
		},
	}
	ctrl := &mockController{
		execResp: &qcs.ExecuteResponse{ExecutionID: "exec-123"},
		statusResp: &qcs.StatusResponse{
			ExecutionID: "exec-123",
			Status:      qcs.StatusSucceeded,
		},
		resultsResp: &qcs.ResultsResponse{
			MemoryValues: map[string][][]int{
				"ro": {
					{0, 0}, // |00>
					{0, 0}, // |00>
					{1, 1}, // |11>
					{1, 1}, // |11>
				},
			},
		},
	}

	b := newTestBackend(t, trans, ctrl)

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).MeasureAll().Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "exec-123" {
		t.Errorf("job ID = %q, want %q", job.ID, "exec-123")
	}
	if job.Backend != "rigetti.Ankaa-3" {
		t.Errorf("backend = %q, want rigetti.Ankaa-3", job.Backend)
	}

	// Check status.
	status, err := b.Status(context.Background(), "exec-123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("status = %s, want completed", status.State)
	}

	// Get results.
	result, err := b.Result(context.Background(), "exec-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts["00"] != 2 {
		t.Errorf("Counts[00] = %d, want 2", result.Counts["00"])
	}
	if result.Counts["11"] != 2 {
		t.Errorf("Counts[11] = %d, want 2", result.Counts["11"])
	}
}

func TestStatusAllStates(t *testing.T) {
	tests := []struct {
		qcsStatus qcs.JobStatus
		want      backend.JobState
	}{
		{qcs.StatusQueued, backend.StateSubmitted},
		{qcs.StatusRunning, backend.StateRunning},
		{qcs.StatusSucceeded, backend.StateCompleted},
		{qcs.StatusFailed, backend.StateFailed},
		{qcs.StatusCanceled, backend.StateCancelled},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.qcsStatus), func(t *testing.T) {
			ctrl := &mockController{
				statusResp: &qcs.StatusResponse{
					ExecutionID: "exec-state",
					Status:      tt.qcsStatus,
				},
			}
			b := newTestBackend(t, &mockTranslation{}, ctrl)
			status, err := b.Status(context.Background(), "exec-state")
			if err != nil {
				t.Fatal(err)
			}
			if status.State != tt.want {
				t.Errorf("mapQCSStatus(%d) = %v, want %v", tt.qcsStatus, status.State, tt.want)
			}
		})
	}
}

func TestStatusWithError(t *testing.T) {
	ctrl := &mockController{
		statusResp: &qcs.StatusResponse{
			ExecutionID: "exec-fail",
			Status:      qcs.StatusFailed,
			Error:       "circuit compilation failed",
		},
	}
	b := newTestBackend(t, &mockTranslation{}, ctrl)
	status, err := b.Status(context.Background(), "exec-fail")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateFailed {
		t.Errorf("state = %v, want failed", status.State)
	}
	if status.Error != "circuit compilation failed" {
		t.Errorf("error = %q, want %q", status.Error, "circuit compilation failed")
	}
}

func TestResultNotCompleted(t *testing.T) {
	ctrl := &mockController{
		statusResp: &qcs.StatusResponse{
			ExecutionID: "exec-running",
			Status:      qcs.StatusRunning,
		},
	}
	b := newTestBackend(t, &mockTranslation{}, ctrl)
	_, err := b.Result(context.Background(), "exec-running")
	if err == nil {
		t.Fatal("expected error for non-completed job")
	}
}

func TestCancelJob(t *testing.T) {
	ctrl := &mockController{}
	b := newTestBackend(t, &mockTranslation{}, ctrl)
	if err := b.Cancel(context.Background(), "exec-cancel"); err != nil {
		t.Fatal(err)
	}
	if ctrl.cancelCalls != 1 {
		t.Errorf("cancelCalls = %d, want 1", ctrl.cancelCalls)
	}
}

func TestNilCircuit(t *testing.T) {
	b := newTestBackend(t, &mockTranslation{}, &mockController{})
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestZeroShots(t *testing.T) {
	b := newTestBackend(t, &mockTranslation{}, &mockController{})
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestPulseProgramError(t *testing.T) {
	b := newTestBackend(t, &mockTranslation{}, &mockController{})
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		PulseProgram: nil, // would be set in practice
		Shots:        100,
	})
	// The nil circuit check should catch this.
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBackendName(t *testing.T) {
	b := newTestBackend(t, &mockTranslation{}, &mockController{})
	if got := b.Name(); got != "rigetti.Ankaa-3" {
		t.Errorf("Name() = %q, want rigetti.Ankaa-3", got)
	}
}

func TestBackendTarget(t *testing.T) {
	b := newTestBackend(t, &mockTranslation{}, &mockController{})
	tgt := b.Target()
	if tgt.Name != target.RigettiAnkaa.Name {
		t.Errorf("Target().Name = %q, want %q", tgt.Name, target.RigettiAnkaa.Name)
	}
	if tgt.NumQubits != 84 {
		t.Errorf("Target().NumQubits = %d, want 84", tgt.NumQubits)
	}
}

func TestTranslationError(t *testing.T) {
	trans := &mockTranslation{
		err: fmt.Errorf("translation failed"),
	}
	ctrl := &mockController{}
	b := newTestBackend(t, trans, ctrl)

	c, _ := builder.New("test", 1).H(0).MeasureAll().Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected translation error")
	}
}

func TestExecutionError(t *testing.T) {
	trans := &mockTranslation{
		resp: &qcs.TranslateResponse{
			EncryptedProgram: []byte("encrypted"),
		},
	}
	ctrl := &mockController{
		execErr: fmt.Errorf("execution failed"),
	}
	b := newTestBackend(t, trans, ctrl)

	c, _ := builder.New("test", 1).H(0).MeasureAll().Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected execution error")
	}
}

func TestProcessorTarget(t *testing.T) {
	tests := []struct {
		processor string
		wantName  string
	}{
		{"Ankaa-3", "Rigetti Ankaa-3"},
		{"Ankaa-2", "Rigetti Ankaa-3"},
		{"Unknown", "Rigetti Ankaa-3"}, // default
	}
	for _, tt := range tests {
		got := processorTarget(tt.processor)
		if got.Name != tt.wantName {
			t.Errorf("processorTarget(%q).Name = %q, want %q", tt.processor, got.Name, tt.wantName)
		}
	}
}

func TestMapQCSStatus(t *testing.T) {
	tests := []struct {
		s    qcs.JobStatus
		want backend.JobState
	}{
		{qcs.StatusQueued, backend.StateSubmitted},
		{qcs.StatusRunning, backend.StateRunning},
		{qcs.StatusSucceeded, backend.StateCompleted},
		{qcs.StatusFailed, backend.StateFailed},
		{qcs.StatusCanceled, backend.StateCancelled},
		{qcs.JobStatus(99), backend.StateSubmitted}, // unknown
	}
	for _, tt := range tests {
		if got := mapQCSStatus(tt.s); got != tt.want {
			t.Errorf("mapQCSStatus(%d) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestShotToBitstring(t *testing.T) {
	tests := []struct {
		shot []int
		want string
	}{
		{[]int{0, 0}, "00"},
		{[]int{1, 1}, "11"},
		{[]int{1, 0, 1}, "101"},
		{[]int{0}, "0"},
	}
	for _, tt := range tests {
		got := shotToBitstring(tt.shot)
		if got != tt.want {
			t.Errorf("shotToBitstring(%v) = %q, want %q", tt.shot, got, tt.want)
		}
	}
}

func TestSerializeCircuit(t *testing.T) {
	c, _ := builder.New("test", 2).H(0).CNOT(0, 1).MeasureAll().Build()
	quil, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(quil) == 0 {
		t.Error("expected non-empty Quil string")
	}
}

func TestParseResults(t *testing.T) {
	resp := &qcs.ResultsResponse{
		MemoryValues: map[string][][]int{
			"ro": {
				{0, 0, 0},
				{1, 0, 1},
				{1, 0, 1},
				{0, 1, 0},
			},
		},
	}

	result, err := parseResults(resp, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts["000"] != 1 {
		t.Errorf("Counts[000] = %d, want 1", result.Counts["000"])
	}
	if result.Counts["101"] != 2 {
		t.Errorf("Counts[101] = %d, want 2", result.Counts["101"])
	}
	if result.Counts["010"] != 1 {
		t.Errorf("Counts[010] = %d, want 1", result.Counts["010"])
	}
	if result.Shots != 4 {
		t.Errorf("Shots = %d, want 4", result.Shots)
	}
}

func TestParseResultsEmpty(t *testing.T) {
	_, err := parseResults(&qcs.ResultsResponse{}, nil, 4)
	if err == nil {
		t.Fatal("expected error for empty results")
	}
}
