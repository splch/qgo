package rigetti

import "github.com/splch/goqu/transpile/target"

// processorTargets maps QCS processor IDs to transpilation targets.
var processorTargets = map[string]target.Target{
	"Ankaa-3": target.RigettiAnkaa,
	"Ankaa-2": target.RigettiAnkaa, // same architecture
}

// processorTarget returns the target for a processor ID.
// Returns RigettiAnkaa as default.
func processorTarget(processor string) target.Target {
	if t, ok := processorTargets[processor]; ok {
		return t
	}
	return target.RigettiAnkaa
}
