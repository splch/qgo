package rigetti

import (
	"context"
	"fmt"

	"github.com/splch/qgo/backend/rigetti/internal/qcs"
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

// grpcClient manages gRPC connections to QCS translation and controller services.
type grpcClient struct {
	translation translationAPI
	controller  controllerAPI
	auth        *tokenProvider
	grpcURL     string
}

func newGRPCClient(auth *tokenProvider, grpcURL string) *grpcClient {
	if grpcURL == "" {
		grpcURL = "grpc.qcs.rigetti.com:443"
	}
	return &grpcClient{
		auth:    auth,
		grpcURL: grpcURL,
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
		QuilProgram: quil,
		NumShots:    shots,
		ProcessorID: processor,
	})
}

// execute submits an encrypted job to the controller.
func (c *grpcClient) execute(ctx context.Context, encrypted []byte, processor string) (*qcs.ExecuteResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for execution: %w", err)
	}
	_ = token

	return c.controller.ExecuteControllerJob(ctx, &qcs.ExecuteRequest{
		EncryptedProgram: encrypted,
		ProcessorID:      processor,
	})
}

// status retrieves the current status of an execution.
func (c *grpcClient) status(ctx context.Context, executionID string) (*qcs.StatusResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for status: %w", err)
	}
	_ = token

	return c.controller.GetControllerJobStatus(ctx, &qcs.StatusRequest{
		ExecutionID: executionID,
	})
}

// results retrieves readout data for a completed execution.
func (c *grpcClient) results(ctx context.Context, executionID string) (*qcs.ResultsResponse, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("rigetti: auth for results: %w", err)
	}
	_ = token

	return c.controller.GetControllerJobResults(ctx, &qcs.ResultsRequest{
		ExecutionID: executionID,
	})
}

// cancel requests cancellation of executions.
func (c *grpcClient) cancel(ctx context.Context, executionIDs []string) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	token, err := c.auth.getToken(ctx)
	if err != nil {
		return fmt.Errorf("rigetti: auth for cancel: %w", err)
	}
	_ = token

	_, err = c.controller.CancelControllerJobs(ctx, &qcs.CancelRequest{
		ExecutionIDs: executionIDs,
	})
	return err
}
