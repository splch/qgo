# Goqu Notebooks

Interactive Jupyter notebooks for exploring Goqu with [gonb](https://github.com/janpfeifer/gonb), a Go Jupyter kernel.

## Setup

```bash
pip install jupyter
go install github.com/janpfeifer/gonb@latest && gonb --install
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/gopls@latest
```

## Running

```bash
cd notebooks
jupyter notebook
```

Open any `.ipynb` file and run cells sequentially.

## Notebooks

| Notebook | Topics |
|---|---|
| [01-getting-started](01-getting-started.ipynb) | Circuits, SVG visualization, statevector simulation, GHZ state, circuit stats |
| [02-simulation-and-noise](02-simulation-and-noise.ipynb) | Density matrix, depolarizing/amplitude damping noise, Pauli expectations, Hamiltonians |
| [03-variational-circuits](03-variational-circuits.ipynb) | Symbolic parameters, binding, parameter sweeps, OpenQASM 3.0 round-trip |
| [04-transpilation](04-transpilation.ipynb) | Hardware transpilation, optimization levels, before/after SVG, Compose/Tensor/Inverse |

## How It Works

Each notebook renders circuit diagrams inline using Goqu's SVG output and gonb's display API:

```go
gonbui.DisplaySvg(draw.SVG(circuit))
```

The `go.mod` in this directory uses a `replace` directive to reference the local Goqu source. gonb resolves this via the `-go_work` kernel flag or a local `go.work` file (gitignored).

## Notes

- Variables shared across cells are declared at the package level using `var` blocks
- Imports are placed in their own declaration cells (no `%%` prefix)
- Executable cells start with `%%`
