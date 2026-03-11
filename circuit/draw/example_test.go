package draw_test

import (
	"fmt"
	"math"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/draw"
)

func Example_bell() {
	c, _ := builder.New("bell", 2).
		H(0).CNOT(0, 1).
		Build()
	fmt.Print(draw.String(c))
	// Output:
	// q0: -H---@--
	//          |
	// q1: -----X--
}

func Example_ghz() {
	c, _ := builder.New("ghz", 3).
		H(0).CNOT(0, 1).CNOT(1, 2).
		Build()
	fmt.Print(draw.String(c))
	// Output:
	// q0: -H---@------
	//          |
	// q1: -----X---@--
	//              |
	// q2: ---------X--
}

func Example_parameterized() {
	c, _ := builder.New("rz", 1).
		RZ(math.Pi/4, 0).
		Build()
	fmt.Print(draw.String(c))
	// Output:
	// q0: -RZ(pi/4)--
}
