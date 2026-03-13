// Package rigetti provides a backend for Rigetti Quantum Cloud Services (QCS).
//
// QCS uses gRPC for program translation and execution, so this package
// is a separate Go module to isolate gRPC/protobuf dependencies from
// the zero-dep core.
//
// The submission flow follows the real QCS pipeline (Gateway strategy):
//  1. Translate Quil → encrypted controller job (gRPC)
//  2. Discover gateway endpoint for processor (REST)
//  3. Execute encrypted job via gateway (gRPC)
//  4. Poll status / fetch results (gRPC)
//
// Usage:
//
//	b := rigetti.New(
//	    rigetti.WithProcessor("Ankaa-3"),
//	)
//	job, err := b.Submit(ctx, &backend.SubmitRequest{
//	    Circuit: circuit,
//	    Shots:   1000,
//	})
//
// By default, credentials are read from ~/.qcs/secrets.toml (matching
// PyQuil/qcs-sdk-rust behavior). Override with WithAccessToken().
package rigetti

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/backend/rigetti/internal/qcs"
	"github.com/splch/goqu/transpile/target"
)

var _ backend.Backend = (*Backend)(nil)

// Backend submits and retrieves quantum jobs via the Rigetti QCS gRPC API.
type Backend struct {
	client    *grpcClient
	processor string
	tgt       target.Target
	logger    *slog.Logger
	jobs      sync.Map // executionID → jobMeta
}

type jobMeta struct {
	qubits          int
	shots           int
	readoutMappings map[string]string
}

// Option configures a Rigetti QCS Backend.
type Option func(*backendConfig)

type backendConfig struct {
	processor   string
	grpcURL     string
	accessToken string
	credPath    string
	logger      *slog.Logger
	// For testing: inject mock services.
	translation translationAPI
	controller  controllerAPI
	accessor    accessorAPI
}

// WithProcessor sets the QPU processor ID (default: "Ankaa-3").
func WithProcessor(name string) Option {
	return func(c *backendConfig) { c.processor = name }
}

// WithGRPCURL overrides the gRPC endpoint.
func WithGRPCURL(url string) Option {
	return func(c *backendConfig) { c.grpcURL = url }
}

// WithAccessToken uses a pre-fetched access token (skips refresh flow).
func WithAccessToken(token string) Option {
	return func(c *backendConfig) { c.accessToken = token }
}

// WithCredentialsPath overrides the default ~/.qcs/ credentials location.
func WithCredentialsPath(path string) Option {
	return func(c *backendConfig) { c.credPath = path }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *backendConfig) { c.logger = l }
}

// withTranslation injects a mock translation service (for testing).
func withTranslation(t translationAPI) Option {
	return func(c *backendConfig) { c.translation = t }
}

// withController injects a mock controller service (for testing).
func withController(ctrl controllerAPI) Option {
	return func(c *backendConfig) { c.controller = ctrl }
}

// withAccessor injects a mock accessor service (for testing).
func withAccessor(acc accessorAPI) Option {
	return func(c *backendConfig) { c.accessor = acc }
}

// New creates a Rigetti QCS backend.
// Credentials are read from ~/.qcs/secrets.toml unless WithAccessToken is used.
func New(opts ...Option) (*Backend, error) {
	cfg := &backendConfig{
		processor: "Ankaa-3",
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var auth *tokenProvider
	if cfg.accessToken != "" {
		auth = newTokenProviderWithToken(cfg.accessToken)
	} else {
		var err error
		auth, err = newTokenProvider(cfg.credPath)
		if err != nil {
			return nil, err
		}
	}

	client := newGRPCClient(auth, cfg.grpcURL)

	// Inject mock services if provided.
	if cfg.translation != nil {
		client.translation = cfg.translation
	}
	if cfg.controller != nil {
		client.controller = cfg.controller
	}
	if cfg.accessor != nil {
		client.accessor = cfg.accessor
	}

	return &Backend{
		client:    client,
		processor: cfg.processor,
		tgt:       processorTarget(cfg.processor),
		logger:    cfg.logger,
	}, nil
}

func (b *Backend) Name() string          { return "rigetti." + b.processor }
func (b *Backend) Target() target.Target { return b.tgt }

// Submit sends a circuit to Rigetti QCS for execution.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
	if req.PulseProgram != nil {
		return nil, fmt.Errorf("rigetti: pulse programs not yet supported (Quil-T support planned)")
	}
	if req.Circuit == nil {
		return nil, fmt.Errorf("rigetti: nil circuit")
	}
	if req.Shots <= 0 {
		return nil, fmt.Errorf("rigetti: shots must be positive")
	}

	quil, err := serializeCircuit(req.Circuit)
	if err != nil {
		return nil, err
	}

	b.logger.InfoContext(ctx, "submitting to Rigetti QCS",
		slog.String("processor", b.processor),
		slog.Int("shots", req.Shots),
		slog.Int("qubits", req.Circuit.NumQubits()),
	)

	// Step 1: Translate Quil to encrypted controller job.
	translateResp, err := b.client.translate(ctx, quil, b.processor, req.Shots)
	if err != nil {
		return nil, fmt.Errorf("rigetti: translate: %w", err)
	}

	// Step 2: Discover gateway endpoint for processor (Gateway strategy).
	// The real qcs-sdk-rust discovers the per-QPU gRPC endpoint via the
	// REST API (GET /v1/quantumProcessors/{id}/accessors) rather than
	// the legacy engagement flow.
	accessor, err := b.client.getAccessor(ctx, b.processor)
	if err != nil {
		return nil, fmt.Errorf("rigetti: accessor lookup: %w", err)
	}

	b.logger.InfoContext(ctx, "gateway discovered",
		slog.String("address", accessor.Address),
	)

	// Step 3: Execute the encrypted job via gateway.
	execResp, err := b.client.execute(ctx, translateResp.Job, b.processor)
	if err != nil {
		return nil, fmt.Errorf("rigetti: execute: %w", err)
	}

	if len(execResp.JobExecutionIDs) == 0 {
		return nil, fmt.Errorf("rigetti: no execution IDs returned")
	}
	executionID := execResp.JobExecutionIDs[0]

	// Extract readout mappings from translation metadata.
	var readoutMappings map[string]string
	if translateResp.Metadata != nil {
		readoutMappings = translateResp.Metadata.ReadoutMappings
	}

	// Store metadata for result retrieval.
	b.jobs.Store(executionID, jobMeta{
		qubits:          req.Circuit.NumQubits(),
		shots:           req.Shots,
		readoutMappings: readoutMappings,
	})

	b.logger.InfoContext(ctx, "job submitted to Rigetti QCS",
		slog.String("execution_id", executionID),
	)

	return &backend.Job{
		ID:      executionID,
		Backend: b.Name(),
		State:   backend.StateSubmitted,
	}, nil
}

// Status returns the current state of a job.
func (b *Backend) Status(ctx context.Context, jobID string) (*backend.JobStatus, error) {
	resp, err := b.client.status(ctx, b.processor, jobID)
	if err != nil {
		return nil, err
	}

	status := &backend.JobStatus{
		ID:       resp.JobID,
		State:    mapQCSStatus(resp.Status),
		Progress: -1,
		QueuePos: -1,
	}
	if resp.Error != "" {
		status.Error = resp.Error
	}
	if status.State == backend.StateCompleted {
		status.Progress = 1.0
	}
	return status, nil
}

// Result retrieves the measurement results from a completed job.
func (b *Backend) Result(ctx context.Context, jobID string) (*backend.Result, error) {
	// Check status first.
	statusResp, err := b.client.status(ctx, b.processor, jobID)
	if err != nil {
		return nil, err
	}
	if mapQCSStatus(statusResp.Status) != backend.StateCompleted {
		return nil, fmt.Errorf("rigetti: job %s is %v, not completed", jobID, statusResp.Status)
	}

	// Fetch results.
	resultsResp, err := b.client.results(ctx, b.processor, jobID)
	if err != nil {
		return nil, fmt.Errorf("rigetti: fetch results: %w", err)
	}

	// Get cached metadata.
	var readoutMappings map[string]string
	var shots int
	if v, ok := b.jobs.Load(jobID); ok {
		meta := v.(jobMeta)
		readoutMappings = meta.readoutMappings
		shots = meta.shots
	}

	return parseResults(resultsResp, readoutMappings, shots)
}

// Cancel requests cancellation of a job.
func (b *Backend) Cancel(ctx context.Context, jobID string) error {
	return b.client.cancel(ctx, b.processor, []string{jobID})
}

// mapQCSStatus converts QCS job status to backend.JobState.
// QCS status values: 0=Unknown, 1=Queued, 2=Running, 3=Succeeded,
// 4=Failed, 5=Canceled, 6=PostProcessing.
func mapQCSStatus(s qcs.JobStatus) backend.JobState {
	switch s {
	case qcs.StatusQueued:
		return backend.StateSubmitted
	case qcs.StatusRunning, qcs.StatusPostProcessing:
		return backend.StateRunning
	case qcs.StatusSucceeded:
		return backend.StateCompleted
	case qcs.StatusFailed:
		return backend.StateFailed
	case qcs.StatusCanceled:
		return backend.StateCancelled
	default:
		// StatusUnknown and any future values.
		return backend.StateSubmitted
	}
}
