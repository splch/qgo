// Package target defines hardware target descriptions for transpilation.
//
// A [Target] specifies the number of physical qubits, allowed basis gates,
// qubit connectivity (nil means all-to-all), and optional fidelity data.
//
// Predefined targets include [IonQForte], [IonQAria], [IBMEagle],
// [QuantinuumH1], [QuantinuumH2], [RigettiAnkaa], [GoogleWillow],
// [GoogleSycamore], and [Simulator] (accepts all gates).
package target
