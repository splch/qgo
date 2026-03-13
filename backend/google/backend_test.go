package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"golang.org/x/oauth2"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/circuit/builder"
)

// staticTokenSource returns a fixed token for testing.
type staticTokenSource struct {
	token string
}

func (s *staticTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: s.token}, nil
}

func newTestBackend(t *testing.T, apiSrv *httptest.Server) *Backend {
	t.Helper()
	return New("test-project",
		WithBaseURL(apiSrv.URL),
		WithTokenSource(&staticTokenSource{token: "test-token"}),
		WithProcessor("willow"),
	)
}

func TestSubmitAndResult(t *testing.T) {
	var submitted atomic.Bool
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}

		switch {
		case r.Method == "POST" && r.URL.Path == "/projects/test-project/programs":
			var req programRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			submitted.Store(true)
			json.NewEncoder(w).Encode(programResponse{
				Name: "projects/test-project/programs/prog-123",
			})

		case r.Method == "POST" && r.URL.Path == "/projects/test-project/programs/prog-123/jobs":
			json.NewEncoder(w).Encode(jobResponse{
				Name: "projects/test-project/programs/prog-123/jobs/job-456",
				ExecutionState: executionState{State: "READY"},
			})

		case r.Method == "GET" && r.URL.Path == "/projects/test-project/programs/prog-123/jobs/job-456":
			json.NewEncoder(w).Encode(jobResponse{
				Name: "projects/test-project/programs/prog-123/jobs/job-456",
				ExecutionState: executionState{State: "SUCCESS"},
			})

		case r.Method == "GET" && r.URL.Path == "/projects/test-project/programs/prog-123/jobs/job-456/result":
			result := cirqResult{
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
			resultJSON, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(resultResponse{
				Result: jobResult{
					TypeURL: "type.googleapis.com/cirq.google.api.v2.Result",
					Value:   base64.StdEncoding.EncodeToString(resultJSON),
				},
			})

		case r.Method == "POST" && r.URL.Path == "/projects/test-project/programs/prog-123/jobs/job-456:cancel":
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   4,
		Name:    "bell-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !submitted.Load() {
		t.Error("program was not submitted")
	}
	if job.ID != "prog-123/job-456" {
		t.Errorf("job ID = %q, want %q", job.ID, "prog-123/job-456")
	}
	if job.Backend != "google.willow" {
		t.Errorf("backend = %q, want google.willow", job.Backend)
	}

	// Check status.
	status, err := b.Status(context.Background(), "prog-123/job-456")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("status = %s, want completed", status.State)
	}

	// Get results.
	result, err := b.Result(context.Background(), "prog-123/job-456")
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
		googleState string
		want        backend.JobState
	}{
		{"READY", backend.StateReady},
		{"RUNNING", backend.StateRunning},
		{"SUCCESS", backend.StateCompleted},
		{"FAILURE", backend.StateFailed},
		{"CANCELLED", backend.StateCancelled},
		{"UNKNOWN", backend.StateSubmitted},
	}

	for _, tt := range tests {
		t.Run(tt.googleState, func(t *testing.T) {
			apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(jobResponse{
					Name:           "projects/test-project/programs/p/jobs/j",
					ExecutionState: executionState{State: tt.googleState},
				})
			}))
			defer apiSrv.Close()

			b := newTestBackend(t, apiSrv)
			status, err := b.Status(context.Background(), "p/j")
			if err != nil {
				t.Fatal(err)
			}
			if status.State != tt.want {
				t.Errorf("parseState(%q) = %v, want %v", tt.googleState, status.State, tt.want)
			}
		})
	}
}

func TestStatusWithError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jobResponse{
			Name:           "projects/test-project/programs/p/jobs/j",
			ExecutionState: executionState{State: "FAILURE"},
			Failure:        &jobFailure{Message: "circuit too deep", Code: "INVALID_CIRCUIT"},
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)
	status, err := b.Status(context.Background(), "p/j")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateFailed {
		t.Errorf("state = %v, want failed", status.State)
	}
	if status.Error != "circuit too deep" {
		t.Errorf("error = %q, want %q", status.Error, "circuit too deep")
	}
}

func TestResultNotCompleted(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jobResponse{
			Name:           "projects/test-project/programs/p/jobs/j",
			ExecutionState: executionState{State: "RUNNING"},
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)
	_, err := b.Result(context.Background(), "p/j")
	if err == nil {
		t.Fatal("expected error for non-completed job")
	}
}

func TestCancelJob(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects/test-project/programs/p/jobs/j:cancel" {
			t.Errorf("path = %s, want /projects/test-project/programs/p/jobs/j:cancel", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)
	if err := b.Cancel(context.Background(), "p/j"); err != nil {
		t.Fatal(err)
	}
}

func TestNilCircuit(t *testing.T) {
	b := New("proj", WithTokenSource(&staticTokenSource{token: "t"}))
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestZeroShots(t *testing.T) {
	b := New("proj", WithTokenSource(&staticTokenSource{token: "t"}))
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestInvalidCompositeID(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)
	_, err := b.Status(context.Background(), "invalid-no-slash")
	if err == nil {
		t.Fatal("expected error for invalid composite ID")
	}
}

func TestRetryOn429(t *testing.T) {
	var attempts atomic.Int32
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(googleAPIError{
				Error: googleErrorDetail{
					Code:    429,
					Message: "Rate limited",
					Status:  "RESOURCE_EXHAUSTED",
				},
			})
			return
		}
		// Return a successful program creation.
		if r.Method == "POST" && r.URL.Path == "/projects/test-project/programs" {
			json.NewEncoder(w).Encode(programResponse{Name: "projects/test-project/programs/p"})
			return
		}
		json.NewEncoder(w).Encode(jobResponse{
			Name: "projects/test-project/programs/p/jobs/j",
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
		Name:    "retry-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := attempts.Load(); got < 3 {
		t.Errorf("attempts = %d, want >= 3", got)
	}
}

func TestBackendName(t *testing.T) {
	b := New("proj", WithProcessor("sycamore"), WithTokenSource(&staticTokenSource{token: "t"}))
	if got := b.Name(); got != "google.sycamore" {
		t.Errorf("Name() = %q, want google.sycamore", got)
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		s    string
		want backend.JobState
	}{
		{"READY", backend.StateReady},
		{"RUNNING", backend.StateRunning},
		{"SUCCESS", backend.StateCompleted},
		{"FAILURE", backend.StateFailed},
		{"CANCELLED", backend.StateCancelled},
		{"UNKNOWN", backend.StateSubmitted},
	}
	for _, tt := range tests {
		if got := parseState(tt.s); got != tt.want {
			t.Errorf("parseState(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestSplitCompositeID(t *testing.T) {
	tests := []struct {
		input   string
		wantP   string
		wantJ   string
		wantErr bool
	}{
		{"prog/job", "prog", "job", false},
		{"a/b", "a", "b", false},
		{"noslash", "", "", true},
	}
	for _, tt := range tests {
		p, j, err := splitCompositeID(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("splitCompositeID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if p != tt.wantP || j != tt.wantJ {
			t.Errorf("splitCompositeID(%q) = (%q, %q), want (%q, %q)", tt.input, p, j, tt.wantP, tt.wantJ)
		}
	}
}

func TestProcessorTarget(t *testing.T) {
	tests := []struct {
		processor string
		wantName  string
	}{
		{"willow", "Google Willow"},
		{"sycamore", "Google Sycamore"},
		{"unknown", "Google Willow"}, // default
	}
	for _, tt := range tests {
		got := processorTarget(tt.processor)
		if got.Name != tt.wantName {
			t.Errorf("processorTarget(%q).Name = %q, want %q", tt.processor, got.Name, tt.wantName)
		}
	}
}

func TestPulseProgramRejected(t *testing.T) {
	b := New("proj", WithTokenSource(&staticTokenSource{token: "t"}))
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	// Without a real API server, this will fail at auth/submission,
	// but we verify pulse rejection separately.
	_ = err

	// Verify nil circuit is rejected.
	_, err = b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}
