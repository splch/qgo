package rigetti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/splch/goqu/backend/rigetti/internal/qcs"
)

const (
	defaultGRPCURL = "grpc.qcs.rigetti.com:443"
	defaultRESTURL = "https://api.qcs.rigetti.com"
)

// translationAPI abstracts the QCS translation gRPC service for testability.
type translationAPI interface {
	TranslateQuilToEncryptedControllerJob(ctx context.Context, req *qcs.TranslateRequest) (*qcs.TranslateResponse, error)
}

// controllerAPI abstracts the QCS controller gRPC service for testability.
type controllerAPI interface {
	ExecuteControllerJob(ctx context.Context, req *qcs.ExecuteRequest) (*qcs.ExecuteResponse, error)
	GetControllerJobStatus(ctx context.Context, req *qcs.StatusRequest) (*qcs.StatusResponse, error)
	GetControllerJobResults(ctx context.Context, req *qcs.ResultsRequest) (*qcs.ResultsResponse, error)
	CancelControllerJobs(ctx context.Context, req *qcs.CancelRequest) (*qcs.CancelResponse, error)
}

// accessorAPI abstracts the QCS REST accessor endpoint for testability.
// The default connection strategy (Gateway) discovers per-QPU gRPC endpoints
// via the REST API rather than the legacy engagement flow.
type accessorAPI interface {
	GetAccessor(ctx context.Context, processor string) (*qcs.AccessorInfo, error)
}

// restAccessorClient implements accessorAPI using the QCS REST API.
type restAccessorClient struct {
	baseURL    string
	httpClient *http.Client
	auth       *tokenProvider
}

// accessorResponse is the JSON response from GET /v1/quantumProcessors/{id}/accessors.
type accessorResponse struct {
	Accessors []struct {
		AccessorType string `json:"accessorType"`
		URL          string `json:"url"`
	} `json:"accessors"`
}

func (c *restAccessorClient) GetAccessor(ctx context.Context, processor string) (*qcs.AccessorInfo, error) {
	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for accessor lookup: %w", err)
	}

	url := fmt.Sprintf("%s/v1/quantumProcessors/%s/accessors", c.baseURL, processor)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("rigetti: create accessor request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rigetti: accessor request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("rigetti: read accessor response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rigetti: accessor lookup failed (%d): %s", resp.StatusCode, string(body))
	}

	var accResp accessorResponse
	if err := json.Unmarshal(body, &accResp); err != nil {
		return nil, fmt.Errorf("rigetti: unmarshal accessor response: %w", err)
	}

	// Find the gateway accessor.
	for _, a := range accResp.Accessors {
		if a.AccessorType == "GATEWAY_V1" || a.AccessorType == "gateway_v1" {
			return &qcs.AccessorInfo{Address: a.URL}, nil
		}
	}

	// Fall back to first available accessor.
	if len(accResp.Accessors) > 0 {
		return &qcs.AccessorInfo{Address: accResp.Accessors[0].URL}, nil
	}

	return nil, fmt.Errorf("rigetti: no accessors found for processor %s", processor)
}

// grpcClient manages gRPC connections to QCS translation and controller services.
type grpcClient struct {
	translation translationAPI
	controller  controllerAPI
	accessor    accessorAPI
	auth        *tokenProvider
	grpcURL     string
	restURL     string
}

func newGRPCClient(auth *tokenProvider, grpcURL string) *grpcClient {
	if grpcURL == "" {
		grpcURL = defaultGRPCURL
	}
	return &grpcClient{
		auth:    auth,
		grpcURL: grpcURL,
		restURL: defaultRESTURL,
		accessor: &restAccessorClient{
			baseURL:    defaultRESTURL,
			httpClient: &http.Client{Timeout: 30 * time.Second},
			auth:       auth,
		},
	}
}

// ensureConnected establishes gRPC connections lazily on first use.
// In production this would dial the gRPC server; for now it checks
// that mock implementations have been injected or returns an error.
func (c *grpcClient) ensureConnected() error {
	if c.translation == nil || c.controller == nil {
		return fmt.Errorf("rigetti: gRPC connection not established (inject mock services for testing, or provide gRPC dial implementation)")
	}
	return nil
}

// translate converts a Quil program to an encrypted controller job.
func (c *grpcClient) translate(ctx context.Context, quil string, processor string, shots int) (*qcs.TranslateResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	// Attach auth token to context (in production, use grpc.WithPerRPCCredentials).
	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for translation: %w", err)
	}
	_ = token // would be injected via gRPC metadata

	return c.translation.TranslateQuilToEncryptedControllerJob(ctx, &qcs.TranslateRequest{
		QuilProgram:        quil,
		NumShots:           shots,
		QuantumProcessorID: processor,
	})
}

// getAccessor discovers the gateway gRPC endpoint for a processor
// via the QCS REST API. This is the default connection strategy
// (Gateway) used by the real qcs-sdk-rust.
func (c *grpcClient) getAccessor(ctx context.Context, processor string) (*qcs.AccessorInfo, error) {
	return c.accessor.GetAccessor(ctx, processor)
}

// execute submits an encrypted job to the controller.
func (c *grpcClient) execute(ctx context.Context, job *qcs.EncryptedControllerJob, processor string) (*qcs.ExecuteResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for execution: %w", err)
	}
	_ = token

	return c.controller.ExecuteControllerJob(ctx, &qcs.ExecuteRequest{
		EncryptedControllerJob: job,
		QuantumProcessorID:     processor,
	})
}

// status retrieves the current status of an execution.
func (c *grpcClient) status(ctx context.Context, processor string, jobID string) (*qcs.StatusResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for status: %w", err)
	}
	_ = token

	return c.controller.GetControllerJobStatus(ctx, &qcs.StatusRequest{
		QuantumProcessorID: processor,
		JobID:              jobID,
	})
}

// results retrieves readout data for a completed execution.
func (c *grpcClient) results(ctx context.Context, processor string, jobID string) (*qcs.ResultsResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for results: %w", err)
	}
	_ = token

	return c.controller.GetControllerJobResults(ctx, &qcs.ResultsRequest{
		QuantumProcessorID: processor,
		JobExecutionID:     jobID,
	})
}

// cancel requests cancellation of executions.
func (c *grpcClient) cancel(ctx context.Context, processor string, jobIDs []string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return fmt.Errorf("rigetti: auth for cancel: %w", err)
	}
	_ = token

	_, err = c.controller.CancelControllerJobs(ctx, &qcs.CancelRequest{
		QuantumProcessorID: processor,
		JobIDs:             jobIDs,
	})
	return err
}
