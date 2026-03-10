// Package pipeline provides pre-built transpilation pipelines.
package pipeline

import (
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile"
	"github.com/splch/qgo/transpile/pass"
	"github.com/splch/qgo/transpile/routing"
	"github.com/splch/qgo/transpile/target"
)

// Level controls the optimization aggressiveness.
type Level int

const (
	LevelNone     Level = 0 // decompose only
	LevelBasic    Level = 1 // + cancel + merge
	LevelFull     Level = 2 // + commute + parallelize
	LevelParallel Level = 3 // multi-strategy, pick best
)

// DefaultPipeline returns a transpilation pass for the given optimization level.
func DefaultPipeline(level Level) transpile.Pass {
	switch level {
	case LevelNone:
		return transpile.Pipeline(
			pass.RemoveBarriers,
			pass.DecomposeToTarget,
			pass.ValidateTarget,
		)
	case LevelBasic:
		return transpile.Pipeline(
			pass.RemoveBarriers,
			routeIfNeeded,
			pass.DecomposeToTarget,
			pass.CancelAdjacent,
			pass.MergeRotations,
			pass.CancelAdjacent,
			pass.ValidateTarget,
		)
	case LevelFull:
		return transpile.Pipeline(
			pass.RemoveBarriers,
			routeIfNeeded,
			pass.DecomposeToTarget,
			pass.CancelAdjacent,
			pass.MergeRotations,
			pass.CommuteThroughCNOT,
			pass.CancelAdjacent,
			pass.MergeRotations,
			pass.ParallelizeOps,
			pass.ValidateTarget,
		)
	case LevelParallel:
		return func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
			strategies := []transpile.Pass{
				DefaultPipeline(LevelBasic),
				DefaultPipeline(LevelFull),
			}
			return OptimizeParallel(c, t, strategies, DefaultCost)
		}
	default:
		return DefaultPipeline(LevelBasic)
	}
}

// DefaultCost scores a circuit: lower is better.
// Weights: 10·TwoQubitGates + Depth + 0.1·GateCount.
func DefaultCost(c *ir.Circuit) float64 {
	s := c.Stats()
	return 10*float64(s.TwoQubitGates) + float64(s.Depth) + 0.1*float64(s.GateCount)
}

// OptimizeParallel runs multiple strategies and returns the lowest-cost result.
func OptimizeParallel(c *ir.Circuit, t target.Target, strategies []transpile.Pass, cost func(*ir.Circuit) float64) (*ir.Circuit, error) {
	type result struct {
		circuit *ir.Circuit
		cost    float64
		err     error
	}

	ch := make(chan result, len(strategies))
	for _, s := range strategies {
		go func(p transpile.Pass) {
			out, err := p(c, t)
			if err != nil {
				ch <- result{err: err}
				return
			}
			ch <- result{circuit: out, cost: cost(out)}
		}(s)
	}

	var best *ir.Circuit
	bestCost := 0.0
	var firstErr error

	for range strategies {
		r := <-ch
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		if best == nil || r.cost < bestCost {
			best = r.circuit
			bestCost = r.cost
		}
	}

	if best == nil {
		return nil, firstErr
	}
	return best, nil
}

// routeIfNeeded applies SABRE routing only when the target has constrained connectivity.
func routeIfNeeded(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	if t.Connectivity == nil {
		return c, nil
	}
	return routing.Route(c, t)
}
