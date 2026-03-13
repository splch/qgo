package google

import "github.com/splch/qgo/transpile/target"

// Processor name constants.
const (
	ProcessorWillow   = "willow"
	ProcessorSycamore = "sycamore"
)

// processorTarget returns the transpilation target for a given processor name.
func processorTarget(processor string) target.Target {
	switch processor {
	case ProcessorSycamore:
		return target.GoogleSycamore
	default:
		return target.GoogleWillow
	}
}
