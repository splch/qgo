# `qgo`: A Quantum SDK for Go

## Implementation Plan

---

## 1. Vision and Scope

### 1.1 Project Identity

**Name:** `qgo` (pronounced "cue-go")
**Tagline:** *Quantum infrastructure, not quantum notebooks.*

`qgo` is a Go-native quantum SDK designed to occupy the layer between researcher-facing circuit construction (Python's domain) and quantum hardware execution. It targets the platform engineer, the infrastructure builder, and the team that needs to ship reliable, observable, high-throughput quantum job pipelines — not the PhD student sketching ansätze in a Jupyter cell.

### 1.2 Design Principles

1. **Infrastructure-first:** Optimize for job orchestration, multi-backend execution, and observability — not symbolic math or ML integration.
2. **Interop over isolation:** Consume and produce OpenQASM 3.0; don't force users to rewrite circuits in Go.
3. **Explicit over magical:** No implicit global state, no monkey-patching, no runtime type coercion. Go's type system is the contract layer.
4. **Composable passes:** Every transformation on a circuit is a pure function. Passes compose, parallelize, and test trivially.
5. **Observable by default:** Structured logging, OpenTelemetry tracing, and metrics are first-class — not afterthoughts bolted on at v3.

### 1.3 What qgo Is Not

- Not a replacement for Qiskit or PennyLane at the researcher-facing layer.
- Not a QML framework. No automatic differentiation, no gradient computation.
- Not a high-performance simulator. Includes a reference statevector simulator for testing, not for 30+ qubit production runs.
- Not a notebook-first tool. No Jupyter kernel, no inline visualization beyond terminal output.

### 1.4 Target Users

| Persona | Use Case |
|---------|----------|
| Platform engineer at a quantum cloud provider | Build job schedulers, transpilation services, multi-tenant execution pipelines |
| DevOps/SRE for quantum teams | CI/CD for quantum circuits, automated benchmarking, cost monitoring |
| SDK/integration engineer | Backend plugins, cross-provider abstraction, hardware-aware routing |
| Quantum software architect | System design for hybrid classical-quantum applications at scale |

---

## 2. Architecture

### 2.1 Module Layout

```
github.com/splch/qgo
├── circuit/          # Core circuit IR, gate definitions, builder API
│   ├── gate/         # Gate interface + standard gate library
│   ├── ir/           # Circuit intermediate representation
│   └── builder/      # Fluent circuit construction API
├── qasm/             # OpenQASM 3.0 parser and emitter
│   ├── parser/       # QASM → circuit IR
│   └── emitter/      # circuit IR → QASM
├── transpile/        # Transpilation pass framework
│   ├── pass/         # Individual optimization/decomposition passes
│   ├── routing/      # Qubit routing and mapping
│   ├── target/       # Hardware target descriptions
│   └── pipeline/     # Pass composition and scheduling
├── sim/              # Reference simulators
│   ├── statevector/  # Full statevector simulator
│   └── density/      # Density matrix simulator (for noise modeling)
├── backend/          # Backend interface and implementations
│   ├── ionq/         # IonQ cloud backend
│   ├── ibm/          # IBM Quantum backend
│   ├── braket/       # Amazon Braket backend
│   ├── local/        # Local simulator backend
│   └── mock/         # Mock backend for testing
├── job/              # Job lifecycle management
│   ├── manager/      # Concurrent job submission and tracking
│   ├── batch/        # Circuit batching and aggregation
│   ├── retry/        # Retry policies and circuit breakers
│   └── result/       # Result types, histogram, expectation values
├── observe/          # Observability infrastructure
│   ├── trace/        # OpenTelemetry integration
│   ├── metrics/      # Prometheus-compatible metrics
│   └── log/          # Structured logging (slog-based)
├── serial/           # Serialization
│   ├── proto/        # Protobuf definitions and codegen
│   └── json/         # JSON serialization for REST APIs
├── cmd/              # CLI tools
│   └── qgo/          # Main CLI binary
└── bench/            # Benchmarking harness
    ├── suite/        # Standard benchmark circuits
    └── report/       # Benchmark result analysis and reporting
```

### 2.2 Dependency Policy

**Zero non-stdlib dependencies in `circuit/`, `transpile/`, and `sim/`.** These core packages use only the Go standard library and `math/cmplx`. This ensures the circuit IR and transpiler can be embedded anywhere without dependency conflicts.

External dependencies are confined to leaf packages:

| Package | Allowed External Deps |
|---------|----------------------|
| `qasm/parser` | None (hand-written recursive descent) |
| `backend/*` | Provider-specific REST/gRPC client libs |
| `observe/trace` | `go.opentelemetry.io/otel` |
| `observe/metrics` | `github.com/prometheus/client_golang` |
| `serial/proto` | `google.golang.org/protobuf` |
| `cmd/qgo` | `github.com/spf13/cobra` |

---

## 3. Core Circuit Layer

### 3.1 Gate Interface

```go
package gate

// Gate represents a quantum gate operation.
type Gate interface {
    // Name returns the canonical gate name (e.g., "H", "CNOT", "RZ").
    Name() string

    // Qubits returns the number of qubits this gate acts on.
    Qubits() int

    // Matrix returns the unitary matrix as a flat row-major slice.
    // Length is (2^n)^2 where n = Qubits().
    Matrix() []complex128

    // Params returns gate parameters (rotation angles, etc.).
    // Returns nil for non-parameterized gates.
    Params() []float64

    // Inverse returns the adjoint (inverse) of this gate.
    Inverse() Gate

    // Decompose breaks this gate into a sequence of simpler gates
    // targeting the given qubit indices. Returns nil if already primitive.
    Decompose(qubits []int) []Applied
}

// Applied pairs a Gate with specific qubit indices.
type Applied struct {
    Gate   Gate
    Qubits []int
}
```

### 3.2 Standard Gate Library

Implement as package-level singleton values for zero-allocation use:

```go
package gate

var (
    I    = &fixed{name: "I", n: 1, matrix: [...]complex128{1, 0, 0, 1}}
    H    = &fixed{name: "H", n: 1, matrix: ...}
    X    = &fixed{name: "X", n: 1, matrix: ...}
    Y    = &fixed{name: "Y", n: 1, matrix: ...}
    Z    = &fixed{name: "Z", n: 1, matrix: ...}
    S    = &fixed{name: "S", n: 1, matrix: ...}
    T    = &fixed{name: "T", n: 1, matrix: ...}
    CNOT = &fixed{name: "CNOT", n: 2, matrix: ...}
    CZ   = &fixed{name: "CZ", n: 2, matrix: ...}
    SWAP = &fixed{name: "SWAP", n: 2, matrix: ...}
    CCX  = &fixed{name: "CCX", n: 3, matrix: ...}  // Toffoli
)

// Parameterized gate constructors
func RX(theta float64) Gate { ... }
func RY(theta float64) Gate { ... }
func RZ(theta float64) Gate { ... }
func Phase(phi float64) Gate { ... }
func U3(theta, phi, lambda float64) Gate { ... }

// IonQ native gates
func GPI(phi float64) Gate { ... }
func GPI2(phi float64) Gate { ... }
func MS(phi0, phi1 float64) Gate { ... }  // Mølmer-Sørensen
```

### 3.3 Circuit IR

```go
package ir

// Circuit is an immutable sequence of quantum operations with metadata.
type Circuit struct {
    name       string
    numQubits  int
    numClbits  int
    ops        []Operation
    metadata   map[string]string
}

// Operation is a single step in a circuit.
type Operation struct {
    Gate      gate.Gate
    Qubits    []int       // qubit indices
    Clbits    []int       // classical bit indices (for measurement)
    Condition *Condition  // optional classical conditioning
}

// Condition represents classical control flow.
type Condition struct {
    Register string
    Value    int
}

// Stats returns circuit statistics without recomputation.
type Stats struct {
    Depth        int
    GateCount    int
    TwoQubitGate int
    Params       int
}
```

### 3.4 Builder API

```go
package builder

// New creates a circuit builder for n qubits.
func New(name string, nQubits int) *Builder { ... }

// Fluent gate application — returns *Builder for chaining.
func (b *Builder) H(q int) *Builder { ... }
func (b *Builder) X(q int) *Builder { ... }
func (b *Builder) CNOT(control, target int) *Builder { ... }
func (b *Builder) RZ(theta float64, q int) *Builder { ... }
func (b *Builder) Apply(g gate.Gate, qubits ...int) *Builder { ... }
func (b *Builder) Barrier(qubits ...int) *Builder { ... }
func (b *Builder) Measure(qubit, clbit int) *Builder { ... }
func (b *Builder) MeasureAll() *Builder { ... }

// Composition
func (b *Builder) Append(other *ir.Circuit, qubitMap map[int]int) *Builder { ... }
func (b *Builder) Repeat(n int) *Builder { ... }

// Build finalizes and returns an immutable Circuit.
// Returns an error if the circuit is invalid (e.g., out-of-range qubits).
func (b *Builder) Build() (*ir.Circuit, error) { ... }
```

**Usage example:**

```go
bell, err := builder.New("bell", 2).
    H(0).
    CNOT(0, 1).
    MeasureAll().
    Build()
```

---

## 4. OpenQASM 3.0 Interop

### 4.1 Parser

Hand-written recursive-descent parser (no parser generators — keeps the zero-dependency contract and gives full control over error messages).

**Scope for v1.0:**

- Gate declarations and applications
- Qubit/classical bit registers
- Measurement
- Classical `if` conditioning
- `barrier`, `reset`
- `include` directives (with configurable include path resolver)
- Standard header (`stdgates.inc`)

**Deferred to v1.1+:**

- Subroutines and scoped variables
- Classical type system (`int`, `float`, `angle`, `bool`)
- `for`/`while` loops
- Pulse-level calibration (`defcal`)

```go
package parser

// Parse reads OpenQASM 3.0 source and returns a Circuit.
func Parse(source io.Reader, opts ...Option) (*ir.Circuit, error) { ... }

// Option configures parser behavior.
type Option func(*config)

func WithIncludePath(paths ...string) Option { ... }
func WithStrictMode(strict bool) Option { ... }  // reject unknown gates vs. treat as opaque
```

### 4.2 Emitter

```go
package emitter

// Emit writes a Circuit as OpenQASM 3.0 source.
func Emit(c *ir.Circuit, w io.Writer, opts ...Option) error { ... }

// Options
func WithComments(include bool) Option { ... }
func WithGateDefinitions(inline bool) Option { ... }
```

### 4.3 Round-Trip Fidelity

Every commit runs a round-trip test suite: parse a corpus of QASM files, emit back to QASM, re-parse, and verify structural equality. The corpus includes the OpenQASM 3.0 conformance examples plus IonQ-, IBM-, and Rigetti-specific circuit patterns.

---

## 5. Transpilation Framework

### 5.1 Pass Interface

```go
package transpile

// Pass transforms a circuit, potentially targeting specific hardware.
type Pass func(c *ir.Circuit, t target.Target) (*ir.Circuit, error)

// Pipeline composes multiple passes into a single pass.
func Pipeline(passes ...Pass) Pass {
    return func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
        var err error
        for _, p := range passes {
            c, err = p(c, t)
            if err != nil {
                return nil, fmt.Errorf("pass failed: %w", err)
            }
        }
        return c, nil
    }
}
```

### 5.2 Hardware Target Description

```go
package target

// Target describes the capabilities and constraints of a quantum backend.
type Target struct {
    Name           string
    NumQubits      int
    BasisGates     []string          // native gate names
    Connectivity   []QubitPair       // allowed two-qubit interactions (nil = all-to-all)
    GateFidelities map[string]float64
    GateTimes      map[string]time.Duration
    MaxCircuitDepth int              // 0 = unlimited
}

type QubitPair struct {
    Q0, Q1 int
}

// Predefined targets
var (
    IonQForte    = Target{Name: "ionq.forte", BasisGates: []string{"GPI", "GPI2", "MS"}, ...}
    IonQAria     = Target{Name: "ionq.aria", BasisGates: []string{"GPI", "GPI2", "MS"}, ...}
    IBMEagle     = Target{Name: "ibm.eagle", BasisGates: []string{"CX", "ID", "RZ", "SX", "X"}, ...}
    Quantinuum   = Target{Name: "quantinuum.h2", BasisGates: []string{"RZZ", "RZ", "RY"}, ...}
    Simulator    = Target{Name: "local.sim", BasisGates: []string{"*"}, ...}
)
```

### 5.3 Standard Passes (v1.0)

| Pass | Description | Priority |
|------|-------------|----------|
| `pass.DecomposeToTarget` | Decompose arbitrary gates into target basis gates using KAK decomposition for two-qubit gates and Euler decomposition for single-qubit gates | P0 |
| `pass.CancelAdjacent` | Cancel adjacent inverse gate pairs (e.g., H·H, X·X) | P0 |
| `pass.MergeRotations` | Merge consecutive single-qubit rotations on the same qubit | P0 |
| `pass.CommuteThroughCNOT` | Commute single-qubit gates through CNOT to enable further cancellations | P1 |
| `pass.RouteSwap` | Insert SWAP gates to satisfy connectivity constraints (Sabre-style heuristic) | P0 |
| `pass.RemoveBarriers` | Strip barrier instructions before hardware submission | P0 |
| `pass.ParallelizeOps` | Reorder independent operations to minimize depth | P1 |
| `pass.ValidateTarget` | Verify circuit satisfies all target constraints (final pass) | P0 |

### 5.4 Concurrent Multi-Strategy Optimization

```go
package pipeline

// OptimizeParallel runs multiple transpilation strategies concurrently
// and returns the best result according to the provided cost function.
func OptimizeParallel(
    c *ir.Circuit,
    t target.Target,
    strategies []transpile.Pass,
    cost func(*ir.Circuit) float64,
) (*ir.Circuit, error) {
    type result struct {
        circuit *ir.Circuit
        score   float64
        err     error
    }
    results := make(chan result, len(strategies))

    for _, s := range strategies {
        go func(strategy transpile.Pass) {
            out, err := strategy(c, t)
            if err != nil {
                results <- result{err: err}
                return
            }
            results <- result{circuit: out, score: cost(out)}
        }(s)
    }

    var best result
    best.score = math.Inf(1)
    for range strategies {
        r := <-results
        if r.err == nil && r.score < best.score {
            best = r
        }
    }
    if best.circuit == nil {
        return nil, errors.New("all strategies failed")
    }
    return best.circuit, nil
}
```

---

## 6. Reference Simulator

### 6.1 Statevector Simulator

```go
package statevector

// Sim simulates a circuit via full statevector evolution.
type Sim struct {
    numQubits int
    state     []complex128  // length 2^numQubits
}

func New(numQubits int) *Sim { ... }

// Run executes the circuit and returns measurement results.
func (s *Sim) Run(c *ir.Circuit, shots int) (*result.Counts, error) { ... }

// StateVector returns the current statevector (for debugging/testing).
func (s *Sim) StateVector() []complex128 { ... }

// ExpectationValue computes <psi|O|psi> for a Pauli observable.
func (s *Sim) ExpectationValue(observable pauli.Operator) (float64, error) { ... }
```

**Performance targets:**

| Qubits | Target Time (single shot) | Implementation |
|--------|---------------------------|----------------|
| ≤16 | <10ms | Single-threaded, cache-friendly |
| 17–24 | <1s | GOMAXPROCS parallelism across amplitude blocks |
| 25–28 | <60s | Memory-mapped state, parallel gate application |
| >28 | Out of scope | Recommend external simulator |

### 6.2 Density Matrix Simulator

For noise-aware testing. Supports depolarizing, amplitude damping, and readout error channels. Same interface as statevector but with `DensityMatrix() [][]complex128`.

---

## 7. Backend and Job Management

### 7.1 Backend Interface

```go
package backend

// Backend represents a quantum execution target.
type Backend interface {
    // Name returns the backend identifier (e.g., "ionq.aria-1").
    Name() string

    // Target returns hardware constraints for transpilation.
    Target() target.Target

    // Submit sends a transpiled circuit for execution.
    Submit(ctx context.Context, req *SubmitRequest) (*Job, error)

    // Status checks the current state of a job.
    Status(ctx context.Context, jobID string) (*JobStatus, error)

    // Result retrieves completed job results.
    Result(ctx context.Context, jobID string) (*result.Result, error)

    // Cancel attempts to cancel a pending/running job.
    Cancel(ctx context.Context, jobID string) error
}

type SubmitRequest struct {
    Circuit  *ir.Circuit
    Shots    int
    Metadata map[string]string
    Priority Priority
    Tags     []string
}

type JobStatus struct {
    ID        string
    State     JobState  // Queued, Running, Completed, Failed, Cancelled
    Progress  float64   // 0.0–1.0 if available
    QueuePos  int       // position in queue, -1 if unknown
    CreatedAt time.Time
    UpdatedAt time.Time
    Error     string    // populated if State == Failed
}
```

### 7.2 IonQ Backend Implementation

```go
package ionq

type Backend struct {
    client   *http.Client
    apiKey   string
    baseURL  string
    tracer   trace.Tracer
}

func New(apiKey string, opts ...Option) *Backend { ... }

// Options
func WithBaseURL(url string) Option { ... }
func WithHTTPClient(c *http.Client) Option { ... }
func WithTracer(t trace.Tracer) Option { ... }

// Submit transpiles to IonQ native format and submits via REST API.
// Automatically converts circuit IR to IonQ JSON circuit format.
func (b *Backend) Submit(ctx context.Context, req *backend.SubmitRequest) (*backend.Job, error) {
    // 1. Validate circuit against IonQ target
    // 2. Serialize to IonQ JSON format (GPI/GPI2/MS native gates)
    // 3. POST to /v0.1/jobs with OTel span propagation
    // 4. Return Job with provider-specific metadata
}
```

### 7.3 Job Manager

```go
package manager

// Manager handles concurrent job submission, polling, and result aggregation.
type Manager struct {
    backends  map[string]backend.Backend
    pollFreq  time.Duration
    maxConc   int  // max concurrent submissions
    tracer    trace.Tracer
    logger    *slog.Logger
    metrics   *metrics.Registry
}

func New(opts ...Option) *Manager { ... }

// SubmitBatch submits a circuit to one or more backends concurrently.
// Returns a channel that delivers results as they complete.
func (m *Manager) SubmitBatch(
    ctx context.Context,
    circuit *ir.Circuit,
    backends []string,
    shots int,
) <-chan BatchResult { ... }

// SubmitSweep submits a parameterized circuit across a range of parameter values.
func (m *Manager) SubmitSweep(
    ctx context.Context,
    circuit *ir.Circuit,
    params []ParameterSweep,
    backend string,
    shots int,
) <-chan SweepResult { ... }

// Watch returns a channel that delivers status updates for a job.
func (m *Manager) Watch(ctx context.Context, jobID string, backend string) <-chan JobStatus { ... }
```

### 7.4 Retry and Resilience

```go
package retry

type Policy struct {
    MaxAttempts    int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    BackoffFactor  float64
    RetryOn        []JobState      // which failure states trigger retry
    CircuitBreaker *CircuitBreaker // optional; trips after N consecutive failures
}

// DefaultPolicy returns a sensible default:
//   3 attempts, 1s initial delay, 30s max, 2x backoff.
func DefaultPolicy() Policy { ... }

type CircuitBreaker struct {
    Threshold  int           // failures before tripping
    ResetAfter time.Duration // cooldown before half-open
}
```

---

## 8. Observability

### 8.1 Tracing

Every backend submission, transpilation pass, and simulation creates OTel spans:

```
qgo.transpile
├── qgo.transpile.decompose_to_target    [gate_count=47 → 31]
├── qgo.transpile.cancel_adjacent        [cancelled=6]
├── qgo.transpile.merge_rotations        [merged=3]
└── qgo.transpile.validate_target        [valid=true]

qgo.submit
├── qgo.submit.serialize                 [format=ionq_json, bytes=2048]
├── qgo.submit.http_post                 [backend=ionq.aria-1, status=200]
└── qgo.submit.job_created               [job_id=abc-123, queue_pos=4]
```

### 8.2 Metrics

Exported as Prometheus-compatible counters/histograms:

| Metric | Type | Description |
|--------|------|-------------|
| `qgo_jobs_submitted_total` | Counter | Total jobs submitted, labeled by backend and status |
| `qgo_jobs_duration_seconds` | Histogram | End-to-end job duration (submit → result) |
| `qgo_transpile_duration_seconds` | Histogram | Transpilation time, labeled by pass |
| `qgo_transpile_gate_reduction_ratio` | Histogram | Ratio of output/input gate count per transpilation |
| `qgo_circuit_depth` | Histogram | Circuit depth before and after transpilation |
| `qgo_backend_errors_total` | Counter | Backend errors, labeled by backend and error type |
| `qgo_job_queue_position` | Gauge | Current queue position for active jobs |

### 8.3 Structured Logging

Uses Go 1.21+ `slog` with JSON output by default:

```go
logger.Info("job submitted",
    slog.String("backend", "ionq.aria-1"),
    slog.String("job_id", "abc-123"),
    slog.Int("shots", 1000),
    slog.Int("circuit_depth", 42),
    slog.Duration("transpile_time", 12*time.Millisecond),
)
```

---

## 9. Serialization

### 9.1 Protobuf Schema

```protobuf
syntax = "proto3";
package qgo.v1;

message Circuit {
    string name = 1;
    int32 num_qubits = 2;
    int32 num_clbits = 3;
    repeated Operation operations = 4;
    map<string, string> metadata = 5;
}

message Operation {
    string gate_name = 1;
    repeated int32 qubits = 2;
    repeated int32 clbits = 3;
    repeated double params = 4;
    Condition condition = 5;
}

message Condition {
    string register = 1;
    int32 value = 2;
}

message JobResult {
    string job_id = 1;
    string backend = 2;
    map<string, int64> counts = 3;  // bitstring → count
    double execution_time_ms = 4;
    map<string, string> metadata = 5;
}
```

### 9.2 Provider-Specific Serialization

Each backend package implements its own marshaling:

- `ionq.MarshalCircuit(c) → IonQ JSON format`
- `ibm.MarshalQObj(c) → Qobj JSON format`
- `braket.MarshalIR(c) → Braket IR JSON format`

---

## 10. CLI Tool (`qgo`)

### 10.1 Command Structure

```
qgo
├── circuit
│   ├── info <file.qasm>           # Print circuit stats (depth, gates, qubits)
│   ├── draw <file.qasm>           # ASCII circuit diagram in terminal
│   ├── convert <in> -o <out>      # Convert between QASM, protobuf, JSON
│   └── validate <file.qasm> -t <target>  # Check target compatibility
├── transpile
│   ├── run <file.qasm> -t <target> [-o out.qasm]  # Transpile circuit
│   ├── compare <file.qasm> -t <target>             # Compare strategies, show stats
│   └── bench <file.qasm> -t <target>               # Benchmark transpilation time
├── job
│   ├── submit <file.qasm> -b <backend> -s <shots>  # Submit job
│   ├── status <job-id> -b <backend>                 # Check job status
│   ├── result <job-id> -b <backend>                 # Retrieve results
│   ├── cancel <job-id> -b <backend>                 # Cancel job
│   ├── list -b <backend>                            # List recent jobs
│   └── watch <job-id> -b <backend>                  # Live status updates
├── sim
│   ├── run <file.qasm> -s <shots>                   # Run on local simulator
│   └── statevector <file.qasm>                      # Print final statevector
├── backend
│   ├── list                                         # List configured backends
│   ├── info <backend>                               # Show backend target info
│   └── calibration <backend>                        # Fetch current calibration data
└── config
    ├── init                                         # Interactive setup
    ├── set <key> <value>                            # Set config value
    └── show                                         # Display current config
```

### 10.2 Configuration

```yaml
# ~/.qgo/config.yaml
defaults:
  backend: ionq.aria-1
  shots: 1000
  output_format: qasm

backends:
  ionq:
    api_key: ${IONQ_API_KEY}  # env var expansion
    base_url: https://api.ionq.co/v0.1
  ibm:
    api_key: ${IBM_QUANTUM_TOKEN}
    instance: ibm-q/open/main

observability:
  log_level: info
  log_format: json
  otel_endpoint: http://localhost:4317
  metrics_port: 9090

transpile:
  optimization_level: 2  # 0=none, 1=basic, 2=aggressive, 3=parallel multi-strategy
  max_parallel_strategies: 4
```

### 10.3 Terminal Circuit Drawing

```
qgo circuit draw bell.qasm

q0: ─[H]─●─[M]→ c0
          │
q1: ──────X─[M]→ c1

Depth: 3 | Gates: 4 (1×H, 1×CNOT, 2×M) | Qubits: 2
```

---

## 11. Benchmarking Harness

### 11.1 Standard Circuits

Ship a corpus of benchmark circuits as embedded QASM files:

| Circuit | Qubits | Purpose |
|---------|--------|---------|
| GHZ-N | 4, 8, 16, 32 | Scaling behavior for entanglement circuits |
| QFT-N | 4, 8, 16 | Transpilation stress (all-to-all connectivity) |
| QAOA-MaxCut | 8, 12 | Variational/parameterized circuit handling |
| Random-Clifford | 10, 20 | General transpilation throughput |
| VQE-H₂ | 4 | Chemistry circuit patterns |
| Grover-N | 4, 8 | Oracle + diffusion decomposition |

### 11.2 Benchmark Framework

```go
package bench

// RunSuite executes all benchmark circuits against a target and reports results.
func RunSuite(t target.Target, opts ...Option) (*Report, error) { ... }

type Report struct {
    Target       string
    Timestamp    time.Time
    Results      []CircuitResult
    SystemInfo   SystemInfo
}

type CircuitResult struct {
    Name              string
    InputGateCount    int
    OutputGateCount   int
    InputDepth        int
    OutputDepth       int
    TwoQubitGates     int
    TranspileTime     time.Duration
    PassBreakdown     map[string]time.Duration
}
```

### 11.3 CLI Integration

```bash
# Run full benchmark suite
qgo transpile bench --target ionq.forte --output report.json

# Compare two targets
qgo transpile bench --target ionq.forte --target ibm.eagle --compare

# Compare against previous run
qgo transpile bench --target ionq.forte --baseline previous-report.json
```

---

## 12. Testing Strategy

### 12.1 Test Categories

| Category | Location | Run When | Tooling |
|----------|----------|----------|---------|
| Unit tests | `*_test.go` alongside source | Every commit | `go test` |
| Integration tests | `integration/` | PR merge, nightly | Build tag `integration` |
| Round-trip tests | `qasm/testdata/` | Every commit | QASM corpus + golden files |
| Fuzz tests | `*_fuzz_test.go` | Nightly | `go test -fuzz` |
| Benchmark tests | `*_benchmark_test.go` | Weekly, on-demand | `go test -bench` |

### 12.2 Key Test Invariants

- **Transpilation correctness:** For every circuit C and target T, the transpiled circuit must be unitarily equivalent to C (verified by statevector comparison on small circuits).
- **QASM round-trip:** `Parse(Emit(c)) ≡ c` for all circuits in the test corpus.
- **Gate decomposition:** Every custom gate's `Decompose()` output is unitarily equivalent to its `Matrix()`.
- **Deterministic builds:** Same input circuit + target + optimization level = same output circuit. Random seed is explicit and configurable.

### 12.3 Fuzz Testing Targets

- QASM parser (malformed input)
- Circuit builder (random gate sequences, out-of-range qubits)
- Protobuf deserialization (corrupted payloads)
- Transpilation passes (adversarial circuit structures)

---

## 13. Implementation Roadmap

### Phase 1: Foundation (Weeks 1–6)

**Goal:** Circuit IR, builder, QASM parsing, and local simulation working end-to-end.

| Week | Deliverables |
|------|-------------|
| 1–2 | `gate` package with full standard gate library and interface. `ir.Circuit` struct with Stats(). Unit tests for all gate matrices. |
| 3 | `builder` package with fluent API. Compile-time qubit validation. Comprehensive builder tests including error cases. |
| 4–5 | `qasm/parser` for OpenQASM 3.0 core subset. QASM test corpus (50+ files). Round-trip test framework. |
| 5–6 | `sim/statevector` simulator for ≤20 qubits. Verification against known results (Bell states, GHZ, QFT-4). |

**Milestone:** `qgo sim run bell.qasm -s 1000` works.

### Phase 2: Transpilation (Weeks 7–12)

**Goal:** Functional transpilation to IonQ and IBM targets with measurable gate reduction.

| Week | Deliverables |
|------|-------------|
| 7–8 | `transpile` pass framework. `target.Target` struct. KAK and Euler decomposition passes. |
| 9–10 | `pass.CancelAdjacent`, `pass.MergeRotations`, `pass.CommuteThroughCNOT`. IonQ and IBM target definitions. |
| 11 | Sabre-style SWAP routing for connectivity-constrained targets. |
| 12 | `pipeline.OptimizeParallel`. Benchmark harness with standard circuits. Transpilation correctness tests via statevector comparison. |

**Milestone:** `qgo transpile run qft-8.qasm -t ionq.forte` produces valid native-gate circuit with >20% gate reduction vs. naive decomposition.

### Phase 3: Execution Layer (Weeks 13–18)

**Goal:** Submit and retrieve jobs from IonQ and local simulator via unified interface.

| Week | Deliverables |
|------|-------------|
| 13–14 | `backend.Backend` interface. `backend/local` wrapping the statevector simulator. `backend/mock` for testing. |
| 15–16 | `backend/ionq` — full IonQ REST API client with authentication, submission, polling, and result retrieval. |
| 17 | `job/manager` with concurrent submission, polling via goroutines/channels, retry policies. |
| 18 | Protobuf serialization. IonQ JSON format marshaling. End-to-end test: build → transpile → submit → result. |

**Milestone:** `qgo job submit bell.qasm -b ionq.aria-1 -s 1000` submits to real hardware and retrieves results.

### Phase 4: Observability and CLI (Weeks 19–22)

**Goal:** Production-grade observability and polished CLI.

| Week | Deliverables |
|------|-------------|
| 19–20 | OpenTelemetry tracing across all layers. Prometheus metrics. slog-based structured logging. |
| 21–22 | Full CLI implementation via Cobra. Config file handling. Terminal circuit drawing. `qgo backend list/info`. |

**Milestone:** Full `qgo` CLI operational with tracing visible in Jaeger/Grafana.

### Phase 5: Additional Backends and Hardening (Weeks 23–28)

**Goal:** IBM and Braket backends. Fuzz testing. Documentation. Public beta readiness.

| Week | Deliverables |
|------|-------------|
| 23–24 | `backend/ibm` — Qiskit Runtime REST API integration. Qobj serialization. |
| 25–26 | `backend/braket` — Amazon Braket API integration. Braket IR serialization. |
| 27 | Fuzz testing for parser, builder, serialization. Edge case hardening. |
| 28 | API documentation (Go doc). README, examples, quickstart guide. |

**Milestone:** Public beta release: `go install github.com/qgo-sdk/qgo/cmd/qgo@latest` works out of the box with IonQ, IBM, and Braket.

---

## 14. Plugin Architecture for Backend Providers

### 14.1 Design

Backend providers ship their own Go modules that implement `backend.Backend`. The `qgo` CLI discovers them via a registry pattern:

```go
package backend

// Registry maps backend names to factory functions.
var registry = map[string]Factory{}

// Factory creates a Backend from configuration.
type Factory func(cfg map[string]string) (Backend, error)

// Register adds a backend factory. Called from provider init() functions.
func Register(name string, f Factory) {
    registry[name] = f
}

// Get returns a backend by name.
func Get(name string, cfg map[string]string) (Backend, error) {
    f, ok := registry[name]
    if !ok {
        return nil, fmt.Errorf("unknown backend: %s", name)
    }
    return f(cfg)
}
```

### 14.2 Third-Party Backend Example

```go
// In a separate module: github.com/example/qgo-myqpu
package myqpu

import "github.com/qgo-sdk/qgo/backend"

func init() {
    backend.Register("myqpu", func(cfg map[string]string) (backend.Backend, error) {
        return &MyQPUBackend{apiKey: cfg["api_key"]}, nil
    })
}
```

---

## 15. Success Criteria

### 15.1 Performance

- Transpilation of a 100-gate circuit to IonQ native gates in <50ms.
- Concurrent submission of 100 jobs to a backend without goroutine leaks or OOM.
- QASM parsing of a 1000-line file in <10ms.
- CLI cold start to first output in <100ms (single binary, no runtime dependencies).

### 15.2 Correctness

- 100% of transpiled circuits are unitarily equivalent to input (verified on circuits ≤16 qubits).
- Zero QASM round-trip regressions across the 50+ file test corpus.
- All gate decompositions verified against analytic unitary matrices to machine precision (1e-14).

### 15.3 Ecosystem

- Functional backends for IonQ, IBM Quantum, Amazon Braket, and local simulator.
- Published protobuf schema for circuit serialization adopted by at least one downstream tool.
- CLI usable in CI/CD pipelines (non-interactive mode, JSON output, exit codes).

---

## 16. Open Questions

1. **Should `ir.Circuit` be mutable or immutable?** Current plan is immutable (builder returns new Circuit). Mutable would reduce allocations in hot transpilation loops but complicates concurrent access. Decision needed after Phase 2 benchmarking.

2. **How deep should OpenQASM 3.0 support go in v1?** The classical type system and loop constructs add significant parser complexity. If the primary use case is infrastructure (not authoring circuits in Go), basic QASM support may suffice for years.

3. **Should parameterized circuits use `float64` or a symbolic type?** `float64` is simpler and aligns with Go philosophy, but prevents symbolic optimization passes. Could add `Symbolic` as an opt-in generic parameter later.

4. **Backend plugin discovery: init() registration vs. explicit import?** The `init()` pattern is Go-idiomatic but makes dependencies implicit. Explicit `backend.Use(ionq.New(...))` is more obvious but less ergonomic for CLI config-file-driven setup.

5. **Density matrix simulator: is it worth including in v1?** Adds significant implementation surface. Could defer to v1.1 and recommend Qiskit Aer for noisy simulation in the meantime.
