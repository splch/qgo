package densitymatrix_test

import (
	"fmt"
	"math"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/densitymatrix"
	"github.com/splch/goqu/sim/noise"
)

func Example_noisyBell() {
	// Create a Bell circuit.
	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()

	// Create a noise model with 1% depolarizing.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.01))

	// Simulate with noise.
	dm := densitymatrix.New(2)
	dm.WithNoise(nm)
	dm.Evolve(c)

	// Check purity (< 1 for mixed state).
	p := dm.Purity()
	fmt.Printf("Purity < 1: %v\n", p < 1.0-1e-10)

	// Output:
	// Purity < 1: true
}

func Example_purityComparison() {
	c, _ := builder.New("h", 1).H(0).Build()

	// Noiseless: purity = 1.
	dm1 := densitymatrix.New(1)
	dm1.Evolve(c)
	fmt.Printf("Noiseless purity: %.4f\n", dm1.Purity())

	// With noise: purity < 1.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.5))
	dm2 := densitymatrix.New(1)
	dm2.WithNoise(nm)
	dm2.Evolve(c)
	fmt.Printf("Noisy purity: %.4f\n", math.Round(dm2.Purity()*10000)/10000)

	// Output:
	// Noiseless purity: 1.0000
	// Noisy purity: 0.5556
}
