package google

// programRequest is the JSON body for POST .../programs.
type programRequest struct {
	Name string     `json:"name"`
	Code programCode `json:"code"`
}

type programCode struct {
	TypeURL string `json:"@type"`
	Value   string `json:"value"` // base64-encoded Cirq JSON
}

// programResponse is returned by POST .../programs.
type programResponse struct {
	Name string `json:"name"` // projects/{project}/programs/{programID}
}

// jobRequest is the JSON body for POST .../programs/{program}/jobs.
type jobRequest struct {
	Name              string            `json:"name"`
	RunContext        jobRunContext      `json:"run_context"`
	ProcessorName     string            `json:"processor_name"`
	SchedulingConfig  schedulingConfig  `json:"scheduling_config"`
	Labels            map[string]string `json:"labels,omitempty"`
}

type jobRunContext struct {
	TypeURL string `json:"@type"`
	Value   string `json:"value"` // base64-encoded run context JSON
}

type schedulingConfig struct {
	Priority int `json:"priority,omitempty"`
}

// runContext is the Cirq run context specifying repetitions.
type runContext struct {
	Repetitions int `json:"repetitions"`
}

// jobResponse is returned by POST .../programs/{program}/jobs and GET .../jobs/{job}.
type jobResponse struct {
	Name           string         `json:"name"` // projects/{project}/programs/{program}/jobs/{job}
	ExecutionState executionState `json:"execution_state"`
	Failure        *jobFailure    `json:"failure,omitempty"`
}

type executionState struct {
	State string `json:"state"` // READY, RUNNING, SUCCESS, FAILURE, CANCELLED
}

type jobFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// resultResponse is returned by GET .../jobs/{job}/result.
type resultResponse struct {
	Result jobResult `json:"result"`
}

type jobResult struct {
	TypeURL string `json:"@type"`
	Value   string `json:"value"` // base64-encoded result JSON
}

// cirqResult is the decoded result containing measurement data.
type cirqResult struct {
	MeasurementResults []measurementResult `json:"measurement_results"`
}

type measurementResult struct {
	Key         string  `json:"key"`
	Qubit       []int   `json:"qubit_indices"`
	Repetitions int     `json:"repetitions"`
	Results     [][]int `json:"results"` // [repetition][qubit] = 0 or 1
}

// googleAPIError is the standard Google Cloud error response format.
type googleAPIError struct {
	Error googleErrorDetail `json:"error"`
}

type googleErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// cirqProgram represents a Cirq circuit in JSON serialization format.
type cirqProgram struct {
	Type    string        `json:"cirq_type"`
	Moments []cirqMoment  `json:"moments"`
	Qubits  []cirqQubit   `json:"device_qubits,omitempty"`
}

type cirqMoment struct {
	Type       string          `json:"cirq_type"`
	Operations []cirqOperation `json:"operations"`
}

type cirqOperation struct {
	Type  string     `json:"cirq_type"`
	Gate  cirqGate   `json:"gate"`
	Qubits []cirqQubitRef `json:"qubits"`
	Args  map[string]any `json:"args,omitempty"`
}

type cirqGate struct {
	Type     string  `json:"cirq_type"`
	Exponent float64 `json:"exponent,omitempty"`
	PhaseExp float64 `json:"phase_exponent,omitempty"`
	AxisPhaseExp float64 `json:"axis_phase_exponent,omitempty"`
	// For measurement gate
	Key string `json:"key,omitempty"`
}

type cirqQubitRef struct {
	Type string `json:"cirq_type"`
	X    int    `json:"x"`
}

type cirqQubit struct {
	Type string `json:"cirq_type"`
	X    int    `json:"x"`
}
