# Goat

Getting started:

This project requires an installation of Go 1.18 to run. Obtain it from the [official download page](https://go.dev/doc/install) or consult your operating system's package manager.

The Goat project uses code generation, so some files must be generated before the tool can be built.
To do so, run these commands:

```bash
go install golang.org/x/tools/cmd/goimports@latest
go generate ./...
```

Now the Goat tool can be built:

```bash
go build Goat
```

To visualize graphs the `xdot` tool must be installed on your system (but it is not required to run the tool).

## TODO
