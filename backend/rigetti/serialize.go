package rigetti

import (
	"fmt"
	"strings"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/backend/rigetti/internal/qcs"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/quil/emitter"
)

// serializeCircuit converts a circuit IR to a Quil string
// suitable for QCS translation.
func serializeCircuit(c *ir.Circuit) (string, error) {
	quil, err := emitter.EmitString(c)
	if err != nil {
		return "", fmt.Errorf("rigetti: emit Quil: %w", err)
	}
	return quil, nil
}

// parseResults converts QCS readout data into a backend.Result.
// QCS returns per-shot measurement arrays (not pre-aggregated counts).
func parseResults(resp *qcs.ResultsResponse, readoutMap map[string]string, shots int) (*backend.Result, error) {
	if resp == nil || len(resp.MemoryValues) == 0 {
		return nil, fmt.Errorf("rigetti: no results in response")
	}

	// Look for the "ro" readout register.
	roData, ok := resp.MemoryValues["ro"]
	if !ok {
		// Try the first available register.
		for _, v := range resp.MemoryValues {
			roData = v
			break
		}
	}

	if len(roData) == 0 {
		return &backend.Result{
			Counts: make(map[string]int),
			Shots:  shots,
		}, nil
	}

	counts := make(map[string]int)
	for _, shot := range roData {
		bs := shotToBitstring(shot)
		counts[bs]++
	}

	return &backend.Result{
		Counts: counts,
		Shots:  len(roData),
	}, nil
}

// shotToBitstring converts a single shot's measurement results to a bitstring.
// Each element is 0 or 1, ordered MSB-first.
func shotToBitstring(shot []int) string {
	var sb strings.Builder
	sb.Grow(len(shot))
	for _, bit := range shot {
		if bit != 0 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
