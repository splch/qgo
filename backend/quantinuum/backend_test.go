package quantinuum

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/transpile/target"
)

// mockLoginServer returns an httptest.Server that always returns a valid JWT token.
func mockLoginServer() *httptest.Server {
	// JWT with exp claim set far in the future (year 2099).
	// Header: {"alg":"none"}, Payload: {"exp":4102444800}, Signature: empty
	const fakeJWT = "eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{
			IDToken:      fakeJWT,
			RefreshToken: "refresh-token",
		})
	}))
}

// newTestBackend creates a Backend wired to mock login and API servers.
func newTestBackend(t *testing.T, loginSrv, apiSrv *httptest.Server) *Backend {
	t.Helper()
	return New("test@example.com", "test-password",
		WithBaseURL(apiSrv.URL),
		WithLoginURL(loginSrv.URL),
		WithDevice("H1-1"),
	)
}

func TestLoginAndTokenCaching(t *testing.T) {
	var loginCalls atomic.Int32
	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginCalls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("login method = %s, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("login Content-Type = %q, want application/json", ct)
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode login body: %v", err)
		}
		if req.Email != "test@example.com" {
			t.Errorf("email = %q, want test@example.com", req.Email)
		}
		if req.Password != "test-password" {
			t.Errorf("password = %q, want test-password", req.Password)
		}

		const fakeJWT = "eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{
			IDToken:      fakeJWT,
			RefreshToken: "refresh-token",
		})
	}))
	defer loginSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("missing Authorization header")
		}
		json.NewEncoder(w).Encode(jobResponse{Job: "job-tok"})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()

	// First call should trigger login.
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Second call should use cached token (no second login).
	_, err = b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := loginCalls.Load(); got != 1 {
		t.Errorf("login was called %d times, want 1 (token should be cached)", got)
	}
}

func TestSubmitAndResult(t *testing.T) {
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	var submitted atomic.Bool
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/job":
			var req jobRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Machine != "H1-1" {
				t.Errorf("machine = %q, want H1-1", req.Machine)
			}
			if req.Language != "OPENQASM 2.0" {
				t.Errorf("language = %q, want OPENQASM 2.0", req.Language)
			}
			if req.Count != 4 {
				t.Errorf("count = %d, want 4", req.Count)
			}
			if len(req.Program) == 0 {
				t.Error("empty program")
			}
			submitted.Store(true)
			json.NewEncoder(w).Encode(jobResponse{Job: "job-q-123"})

		case r.Method == "GET" && r.URL.Path == "/job/job-q-123":
			json.NewEncoder(w).Encode(jobStatusResponse{
				Job:    "job-q-123",
				Status: "completed",
				Results: map[string]int{
					"00": 2,
					"11": 2,
				},
			})

		case r.Method == "DELETE" && r.URL.Path == "/job/job-q-123":
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)

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
		t.Error("job was not submitted")
	}
	if job.ID != "job-q-123" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-q-123")
	}
	if job.Backend != "quantinuum.H1-1" {
		t.Errorf("backend = %q, want quantinuum.H1-1", job.Backend)
	}

	// Check status.
	status, err := b.Status(context.Background(), "job-q-123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("status = %s, want completed", status.State)
	}

	// Get results.
	result, err := b.Result(context.Background(), "job-q-123")
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
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	tests := []struct {
		qStatus string
		want    backend.JobState
	}{
		{"queued", backend.StateSubmitted},
		{"submitted", backend.StateSubmitted},
		{"running", backend.StateRunning},
		{"completed", backend.StateCompleted},
		{"failed", backend.StateFailed},
		{"cancelling", backend.StateCancelled},
		{"cancelled", backend.StateCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.qStatus, func(t *testing.T) {
			apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(jobStatusResponse{
					Job:    "job-state",
					Status: tt.qStatus,
				})
			}))
			defer apiSrv.Close()

			b := newTestBackend(t, loginSrv, apiSrv)
			status, err := b.Status(context.Background(), "job-state")
			if err != nil {
				t.Fatal(err)
			}
			if status.State != tt.want {
				t.Errorf("parseState(%q) = %v, want %v", tt.qStatus, status.State, tt.want)
			}
		})
	}
}

func TestStatusWithError(t *testing.T) {
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jobStatusResponse{
			Job:    "job-fail",
			Status: "failed",
			Error:  "circuit too deep for device",
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	status, err := b.Status(context.Background(), "job-fail")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateFailed {
		t.Errorf("state = %v, want failed", status.State)
	}
	if status.Error != "circuit too deep for device" {
		t.Errorf("error = %q, want %q", status.Error, "circuit too deep for device")
	}
}

func TestResultNotCompleted(t *testing.T) {
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jobStatusResponse{
			Job:    "job-running",
			Status: "running",
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	_, err := b.Result(context.Background(), "job-running")
	if err == nil {
		t.Fatal("expected error for non-completed job")
	}
}

func TestCancelJob(t *testing.T) {
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/job/job-cancel" {
			t.Errorf("path = %s, want /job/job-cancel", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	if err := b.Cancel(context.Background(), "job-cancel"); err != nil {
		t.Fatal(err)
	}
}

func TestNilCircuit(t *testing.T) {
	b := New("email", "password")
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestZeroShots(t *testing.T) {
	b := New("email", "password")
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestRetryOn429(t *testing.T) {
	loginSrv := mockLoginServer()
	defer loginSrv.Close()

	var attempts atomic.Int32
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(apiErrorResponse{
				Code:    "rate_limit",
				Message: "Rate limited",
			})
			return
		}
		json.NewEncoder(w).Encode(jobResponse{Job: "job-retry"})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-retry" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-retry")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestAuthError(t *testing.T) {
	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Invalid credentials"}`))
	}))
	defer loginSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called when login auth fails")
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, loginSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		s    string
		want backend.JobState
	}{
		{"queued", backend.StateSubmitted},
		{"submitted", backend.StateSubmitted},
		{"running", backend.StateRunning},
		{"completed", backend.StateCompleted},
		{"failed", backend.StateFailed},
		{"cancelling", backend.StateCancelled},
		{"cancelled", backend.StateCancelled},
		{"canceled", backend.StateCancelled},
		{"unknown", backend.StateSubmitted},
	}
	for _, tt := range tests {
		if got := parseState(tt.s); got != tt.want {
			t.Errorf("parseState(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestDeviceTarget(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{"H1-1", "Quantinuum H1"},
		{"H1-2", "Quantinuum H1"},
		{"H1-1E", "Quantinuum H1"},
		{"H1-1SC", "Quantinuum H1"},
		{"H2-1", "Quantinuum H2"},
		{"H2-1E", "Quantinuum H2"},
		{"unknown", "Quantinuum H1"}, // default
	}
	for _, tt := range tests {
		got := deviceTarget(tt.device)
		if got.Name != tt.want {
			t.Errorf("deviceTarget(%q).Name = %q, want %q", tt.device, got.Name, tt.want)
		}
	}
}

func TestBackendName(t *testing.T) {
	b := New("email", "password", WithDevice("H2-1"))
	if got := b.Name(); got != "quantinuum.H2-1" {
		t.Errorf("Name() = %q, want quantinuum.H2-1", got)
	}
}

func TestSerializeCircuit(t *testing.T) {
	c, _ := builder.New("test", 2).H(0).CNOT(0, 1).Build()
	qasm, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(qasm) == 0 {
		t.Error("expected non-empty QASM string")
	}
	// Verify it's QASM 2.0 format.
	if !contains(qasm, "OPENQASM 2.0") {
		t.Error("expected OPENQASM 2.0 header")
	}
	if !contains(qasm, "qreg q[2]") {
		t.Error("expected qreg declaration")
	}
	if !contains(qasm, "qelib1.inc") {
		t.Error("expected qelib1.inc include")
	}
	if !contains(qasm, "h q[0]") {
		t.Error("expected h gate")
	}
	if !contains(qasm, "cx q[0], q[1]") {
		t.Error("expected cx gate")
	}
}

func TestSerializeCircuitWithMeasurement(t *testing.T) {
	c, _ := builder.New("test", 2).H(0).CNOT(0, 1).MeasureAll().Build()
	qasm, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(qasm, "creg c[2]") {
		t.Error("expected creg declaration")
	}
	if !contains(qasm, "measure q[0] -> c[0]") {
		t.Error("expected measure instruction")
	}
}

func TestDeviceTargetH1(t *testing.T) {
	got := deviceTarget("H1-1")
	if got.NumQubits != 20 {
		t.Errorf("H1 NumQubits = %d, want 20", got.NumQubits)
	}
}

func TestDeviceTargetH2(t *testing.T) {
	got := deviceTarget("H2-1")
	if got.NumQubits != 56 {
		t.Errorf("H2 NumQubits = %d, want 56", got.NumQubits)
	}
}

func TestBackendTarget(t *testing.T) {
	b := New("email", "password", WithDevice("H2-1"))
	tgt := b.Target()
	if tgt.Name != "Quantinuum H2" {
		t.Errorf("Target().Name = %q, want Quantinuum H2", tgt.Name)
	}
	if tgt.NumQubits != 56 {
		t.Errorf("Target().NumQubits = %d, want 56", tgt.NumQubits)
	}
}

func TestQuantinuumH1TargetProperties(t *testing.T) {
	tgt := target.QuantinuumH1
	if tgt.NumQubits != 20 {
		t.Errorf("QuantinuumH1.NumQubits = %d, want 20", tgt.NumQubits)
	}
	if tgt.Connectivity != nil {
		t.Error("QuantinuumH1 should have nil connectivity (all-to-all)")
	}
	for _, g := range []string{"RZZ", "RZ", "RY"} {
		if !tgt.HasBasisGate(g) {
			t.Errorf("QuantinuumH1.HasBasisGate(%q) = false, want true", g)
		}
	}
}

func TestQuantinuumH2TargetProperties(t *testing.T) {
	tgt := target.QuantinuumH2
	if tgt.NumQubits != 56 {
		t.Errorf("QuantinuumH2.NumQubits = %d, want 56", tgt.NumQubits)
	}
	if tgt.Connectivity != nil {
		t.Error("QuantinuumH2 should have nil connectivity (all-to-all)")
	}
	for _, g := range []string{"RZZ", "RZ", "RY"} {
		if !tgt.HasBasisGate(g) {
			t.Errorf("QuantinuumH2.HasBasisGate(%q) = false, want true", g)
		}
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
