// Package otelbridge provides OpenTelemetry span hooks for goqu operations.
package otelbridge

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/splch/goqu/observe"
)

const tracerName = "github.com/splch/goqu"

// Option configures the OTel bridge.
type Option func(*config)

type config struct {
	tracer trace.Tracer
}

// WithTracer overrides the default OTel tracer.
func WithTracer(t trace.Tracer) Option {
	return func(c *config) { c.tracer = t }
}

// NewHooks returns observe.Hooks that create OTel spans for all goqu operations.
// Child spans are automatically nested via context propagation.
func NewHooks(opts ...Option) *observe.Hooks {
	cfg := config{tracer: otel.Tracer(tracerName)}
	for _, o := range opts {
		o(&cfg)
	}
	t := cfg.tracer

	return &observe.Hooks{
		WrapTranspile: func(ctx context.Context, level int, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			ctx, span := t.Start(ctx, "goqu.transpile",
				trace.WithAttributes(
					attribute.Int("goqu.transpile.level", level),
					attribute.String("goqu.circuit.name", in.Name),
					attribute.Int("goqu.circuit.qubits", in.NumQubits),
					attribute.Int("goqu.circuit.gates.in", in.GateCount),
					attribute.Int("goqu.circuit.depth.in", in.Depth),
				))
			return ctx, func(out observe.CircuitInfo, err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				} else {
					span.SetAttributes(
						attribute.Int("goqu.circuit.gates.out", out.GateCount),
						attribute.Int("goqu.circuit.depth.out", out.Depth),
						attribute.Int("goqu.circuit.two_qubit.out", out.TwoQubitGates),
					)
				}
				span.End()
			}
		},

		WrapPass: func(ctx context.Context, pass string, in observe.CircuitInfo) (context.Context, func(observe.CircuitInfo, error)) {
			ctx, span := t.Start(ctx, "goqu.transpile."+pass,
				trace.WithAttributes(
					attribute.Int("goqu.circuit.gates.in", in.GateCount),
					attribute.Int("goqu.circuit.depth.in", in.Depth),
				))
			return ctx, func(out observe.CircuitInfo, err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				} else {
					span.SetAttributes(
						attribute.Int("goqu.circuit.gates.out", out.GateCount),
						attribute.Int("goqu.circuit.depth.out", out.Depth),
					)
				}
				span.End()
			}
		},

		WrapJob: func(ctx context.Context, info observe.JobInfo) (context.Context, func(string, error)) {
			ctx, span := t.Start(ctx, "goqu.job",
				trace.WithAttributes(
					attribute.String("goqu.backend", info.Backend),
					attribute.Int("goqu.shots", info.Shots),
					attribute.Int("goqu.qubits", info.Qubits),
				))
			return ctx, func(jobID string, err error) {
				if jobID != "" {
					span.SetAttributes(attribute.String("goqu.job_id", jobID))
				}
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		WrapSim: func(ctx context.Context, info observe.SimInfo) (context.Context, func(error)) {
			ctx, span := t.Start(ctx, "goqu.simulate",
				trace.WithAttributes(
					attribute.Int("goqu.qubits", info.NumQubits),
					attribute.Int("goqu.gates", info.GateCount),
					attribute.Int("goqu.shots", info.Shots),
				))
			return ctx, func(err error) {
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		WrapHTTP: func(ctx context.Context, info observe.HTTPInfo) (context.Context, func(int, error)) {
			ctx, span := t.Start(ctx, "goqu.http",
				trace.WithAttributes(
					attribute.String("http.method", info.Method),
					attribute.String("http.target", info.Path),
					attribute.String("goqu.backend", info.Backend),
				))
			return ctx, func(statusCode int, err error) {
				if statusCode > 0 {
					span.SetAttributes(attribute.Int("http.status_code", statusCode))
				}
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.End()
			}
		},

		OnJobPoll: func(ctx context.Context, info observe.JobPollInfo) {
			span := trace.SpanFromContext(ctx)
			span.AddEvent("goqu.job.poll", trace.WithAttributes(
				attribute.String("goqu.job_id", info.JobID),
				attribute.String("goqu.state", info.State),
				attribute.Int("goqu.attempt", info.Attempt),
			))
		},
	}
}
