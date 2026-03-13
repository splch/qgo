package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend submits and retrieves quantum jobs via the Google Quantum Engine API.
type Backend struct {
	client    *httpClient
	projectID string
	processor string // "willow", "sycamore"
	tgt       target.Target
	jobs      sync.Map // compositeID → jobMeta
	logger    *slog.Logger

	// Auth configuration (resolved during first request if needed).
	credentialsJSON []byte
	tokenSource     oauth2.TokenSource
}

type jobMeta struct {
	qubits int
	shots  int
}

// Option configures a Google Quantum Backend.
type Option func(*Backend)

// WithProcessor sets the Google Quantum processor (default: "willow").
func WithProcessor(processor string) Option {
	return func(b *Backend) { b.processor = processor }
}

// WithBaseURL overrides the Google Quantum Engine API base URL.
func WithBaseURL(url string) Option {
	return func(b *Backend) { b.client.baseURL = url }
}

// WithCredentialsJSON provides a service account JSON key for authentication.
func WithCredentialsJSON(jsonKey []byte) Option {
	return func(b *Backend) { b.credentialsJSON = jsonKey }
}

// WithTokenSource provides a custom oauth2.TokenSource for authentication.
func WithTokenSource(src oauth2.TokenSource) Option {
	return func(b *Backend) { b.tokenSource = src }
}

// WithHTTPClient provides a custom HTTP client for API requests.
func WithHTTPClient(c *http.Client) Option {
	return func(b *Backend) { b.client.base = c }
}

// WithLogger sets the structured logger for the Google backend.
func WithLogger(l *slog.Logger) Option {
	return func(b *Backend) { b.logger = l }
}

// New creates a Google Quantum Engine backend for the given project ID.
//
// Authentication resolves in order: WithTokenSource > WithCredentialsJSON >
// Application Default Credentials (ADC).
func New(projectID string, opts ...Option) *Backend {
	// Create with a placeholder auth; resolved lazily or via options.
	b := &Backend{
		client:    newHTTPClient(nil, "", nil),
		projectID: projectID,
		processor: ProcessorWillow,
		tgt:       target.GoogleWillow,
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(b)
	}
	b.tgt = processorTarget(b.processor)
	b.client.backend = b.Name()
	return b
}

// initAuth initializes the token provider if not already set.
func (b *Backend) initAuth(ctx context.Context) error {
	if b.client.auth != nil {
		return nil
	}

	var (
		tp  *tokenProvider
		err error
	)
	switch {
	case b.tokenSource != nil:
		tp = newTokenProviderFromSource(b.tokenSource)
	case b.credentialsJSON != nil:
		tp, err = newTokenProviderFromJSON(ctx, b.credentialsJSON)
	default:
		tp, err = newTokenProviderFromDefault(ctx)
	}
	if err != nil {
		return err
	}
	b.client.auth = tp
	return nil
}

func (b *Backend) Name() string          { return "google." + b.processor }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit to Google Quantum Engine for execution.
// The API requires two steps: create a Program, then create a Job under it.
// The composite job ID is encoded as "{programID}/{jobID}".
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.PulseProgram != nil {
		return nil, fmt.Errorf("google: pulse programs are not supported")
	}
	if req.Circuit == nil {
		return nil, fmt.Errorf("google: nil circuit")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("google: shots must be positive")
	}

	if err := b.initAuth(ctx); err != nil {
		return nil, err
	}

	// Serialize circuit to Cirq JSON.
	program, err := serializeCircuit(req.Circuit)
	if err != nil {
		return nil, err
	}
	programJSON, err := json.Marshal(program)
	if err != nil {
		return nil, fmt.Errorf("google: marshal program: %w", err)
	}

	b.logger.InfoContext(ctx, "submitting to Google Quantum Engine",
		slog.String("processor", b.processor),
		slog.Int("shots", req.Shots),
		slog.Int("qubits", req.Circuit.NumQubits()),
	)

	// Step 1: Create Program.
	programBody := &programRequest{
		Name: fmt.Sprintf("projects/%s/programs/qgo-%s", b.projectID, req.Name),
		Code: programCode{
			TypeURL: "type.googleapis.com/cirq.google.api.v2.Program",
			Value:   base64.StdEncoding.EncodeToString(programJSON),
		},
	}

	var progResp programResponse
	progPath := fmt.Sprintf("/projects/%s/programs", b.projectID)
	if err := b.client.do(ctx, http.MethodPost, progPath, programBody, &progResp); err != nil {
		return nil, fmt.Errorf("google: create program: %w", err)
	}

	// Extract program ID from the name: projects/{project}/programs/{programID}
	programID := lastSegment(progResp.Name)

	// Step 2: Create Job.
	rcJSON, _ := json.Marshal(runContext{Repetitions: req.Shots})

	jobBody := &jobRequest{
		RunContext: jobRunContext{
			TypeURL: "type.googleapis.com/cirq.google.api.v2.RunContext",
			Value:   base64.StdEncoding.EncodeToString(rcJSON),
		},
		ProcessorName: fmt.Sprintf("projects/%s/processors/%s", b.projectID, b.processor),
	}
	if req.Metadata != nil {
		jobBody.Labels = req.Metadata
	}

	var jobResp jobResponse
	jobPath := fmt.Sprintf("/projects/%s/programs/%s/jobs", b.projectID, programID)
	if err := b.client.do(ctx, http.MethodPost, jobPath, jobBody, &jobResp); err != nil {
		return nil, fmt.Errorf("google: create job: %w", err)
	}

	jobID := lastSegment(jobResp.Name)
	compositeID := programID + "/" + jobID

	b.jobs.Store(compositeID, jobMeta{qubits: req.Circuit.NumQubits(), shots: req.Shots})

	b.logger.InfoContext(ctx, "job submitted to Google Quantum Engine",
		slog.String("job_id", compositeID),
	)

	return &backend.Job{
		ID:      compositeID,
		Backend: b.Name(),
		State:   backend.StateSubmitted,
	}, nil
}

// Status returns the current state of a job.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	if err := b.initAuth(ctx); err != nil {
		return nil, err
	}

	programID, jID, err := splitCompositeID(jobID)
	if err != nil {
		return nil, err
	}

	var resp jobResponse
	path := fmt.Sprintf("/projects/%s/programs/%s/jobs/%s", b.projectID, programID, jID)
	if err := b.client.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	status := &backend.JobStatus{
		ID:       jobID,
		State:    parseState(resp.ExecutionState.State),
		Progress: -1,
		QueuePos: -1,
	}
	if resp.Failure != nil {
		status.Error = resp.Failure.Message
	}
	if status.State == backend.StateCompleted {
		status.Progress = 1.0
	}
	return status, nil
}

// Result retrieves the measurement results from a completed job.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	if err := b.initAuth(ctx); err != nil {
		return nil, err
	}

	programID, jID, err := splitCompositeID(jobID)
	if err != nil {
		return nil, err
	}

	// Check job status first.
	var statusResp jobResponse
	statusPath := fmt.Sprintf("/projects/%s/programs/%s/jobs/%s", b.projectID, programID, jID)
	if err := b.client.do(ctx, http.MethodGet, statusPath, nil, &statusResp); err != nil {
		return nil, err
	}
	if parseState(statusResp.ExecutionState.State) != backend.StateCompleted {
		return nil, fmt.Errorf("google: job %s is %s, not completed", jobID, statusResp.ExecutionState.State)
	}

	// Fetch results.
	var resultResp resultResponse
	resultPath := fmt.Sprintf("/projects/%s/programs/%s/jobs/%s/result", b.projectID, programID, jID)
	if err := b.client.do(ctx, http.MethodGet, resultPath, nil, &resultResp); err != nil {
		return nil, fmt.Errorf("google: fetch results: %w", err)
	}

	// Decode base64 result value.
	resultData, err := base64.StdEncoding.DecodeString(resultResp.Result.Value)
	if err != nil {
		return nil, fmt.Errorf("google: decode result: %w", err)
	}

	var cr cirqResult
	if err := json.Unmarshal(resultData, &cr); err != nil {
		return nil, fmt.Errorf("google: unmarshal result: %w", err)
	}

	shots := 0
	if v, ok := b.jobs.Load(jobID); ok {
		shots = v.(jobMeta).shots
	}

	return parseResults(cr, shots)
}

// Cancel requests cancellation of a job.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	if err := b.initAuth(ctx); err != nil {
		return err
	}

	programID, jID, err := splitCompositeID(jobID)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/projects/%s/programs/%s/jobs/%s:cancel", b.projectID, programID, jID)
	return b.client.do(ctx, http.MethodPost, path, nil, nil)
}

func parseState(s string) backend.JobState {
	switch s {
	case "READY":
		return backend.StateReady
	case "RUNNING":
		return backend.StateRunning
	case "SUCCESS":
		return backend.StateCompleted
	case "FAILURE":
		return backend.StateFailed
	case "CANCELLED":
		return backend.StateCancelled
	default:
		return backend.StateSubmitted
	}
}

// splitCompositeID splits a "{programID}/{jobID}" into its parts.
func splitCompositeID(compositeID string) (programID, jobID string, err error) {
	parts := strings.SplitN(compositeID, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("google: invalid job ID %q (expected programID/jobID)", compositeID)
	}
	return parts[0], parts[1], nil
}

// lastSegment returns the last path segment of a resource name.
func lastSegment(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
