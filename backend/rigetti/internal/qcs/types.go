// Package qcs defines types that mirror QCS gRPC service messages.
// These will be replaced by protoc-generated types once proto files
// are obtained from github.com/rigetti/qcs-sdk-rust.
package qcs

// TranslateRequest is the input to the translation service.
type TranslateRequest struct {
	QuilProgram string
	NumShots    int
	ProcessorID string
}

// TranslateResponse is the output of the translation service.
type TranslateResponse struct {
	EncryptedProgram []byte
	ReadoutMap       map[string]string // qubit index mapping
}

// ExecuteRequest is the input to the controller execution service.
type ExecuteRequest struct {
	EncryptedProgram []byte
	ProcessorID      string
	PatchValues      map[string][]float64 // optional runtime patch values
}

// ExecuteResponse is the output of the controller execution service.
type ExecuteResponse struct {
	ExecutionID string
}

// StatusRequest is the input to the status query service.
type StatusRequest struct {
	ExecutionID string
}

// JobStatus represents the state of a QCS execution.
type JobStatus int

const (
	StatusQueued    JobStatus = 0
	StatusRunning   JobStatus = 1
	StatusSucceeded JobStatus = 2
	StatusFailed    JobStatus = 3
	StatusCanceled  JobStatus = 4
)

// StatusResponse is the output of the status query service.
type StatusResponse struct {
	ExecutionID string
	Status      JobStatus
	Error       string
}

// ResultsRequest is the input to the results query service.
type ResultsRequest struct {
	ExecutionID string
}

// ResultsResponse is the output of the results query service.
type ResultsResponse struct {
	// MemoryValues maps register name to per-shot readout data.
	// For "ro" register: MemoryValues["ro"] is [shots][qubits]int.
	MemoryValues map[string][][]int
}

// CancelRequest is the input to the cancellation service.
type CancelRequest struct {
	ExecutionIDs []string
}

// CancelResponse is the output of the cancellation service.
type CancelResponse struct{}
