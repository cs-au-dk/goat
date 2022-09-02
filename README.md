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

## Running the tool

To run the blocking error detection analysis on the example program `sync-two-goros-race`:

```go
package main

func main() {
    ch := make(chan int)
    go func() {
        ch <- 10
    }()

    go func() {
        ch <- 20
    }()

    <-ch
}
```

Run the following command:

```bash
./Goat -gopath examples -task collect-primitives -metrics -psets gcatch simple-examples/sync-two-goros-race
```

The tool outputs two bug reports:

```
Potential blocked goroutine at superlocation: ⟨  [ main:entry ] ⇒ [ ⊥ ]
  | [ main:entry ] ↝ [ go t2() ] ⇒ [ send t0 <- 10:int ]
  | [ main:entry ] ↝ [ go t3() ] ⇒ [ ⊥ ]
⟩
Goroutine: [ main:entry ] ↝ [ go t2() ]
Control location: [ send t0 <- 10:int ]
Source: examples/src/simple-examples/sync-two-goros-race/main.go:6:6

Potential blocked goroutine at superlocation: ⟨  [ main:entry ] ⇒ [ ⊥ ]
  | [ main:entry ] ↝ [ go t2() ] ⇒ [ ⊥ ]
  | [ main:entry ] ↝ [ go t3() ] ⇒ [ send t0 <- 20:int ]
⟩
Goroutine: [ main:entry ] ↝ [ go t3() ]
Control location: [ send t0 <- 20:int ]
Source: examples/src/simple-examples/sync-two-goros-race/main.go:10:6
```

Both reports are true positives.
Only one of the threads can synchronize with the main goroutine so the other will be blocked forever.
Which thread it is depends on the scheduler and runtime system, but both are possible.

No report is issued for the receive in the main function - this operation will always succeed.

You can add the `-visualize` argument to the command line before specifying the program to be analyzed
to get a visualization of possible program behaviors that lead to bugs.

Other useful command line arguments are:

* `-gopath <PATH>`:
	Controls the value of the `GOPATH` environment variable that will be used when loading the code to be analyzed.
	This path will contain downloaded dependencies if the code to be analyzed has them.
* `-modulepath <PATH>`:
	If the code to be analyzed is organized as a Go module, you can specify this by providing the path to the folder containing the `go.mod` file. The code will then be loaded in Go 1.11 "Module Aware" mode.
* `-include-tests`:
	Necessary if the code to be analyzed is a test.
* `-fun <NAME>`:
	Allows you to specify the name of a single program entry point (a function) that should be analyzed instead of analyzing all entry points (when analyzing tests).

To run the analysis on the `raft` module of [`etcd`](https://github.com/etcd-io/etcd) run the following commands:
```bash
mkdir tmp
git clone https://github.com/etcd-io/etcd --branch release-3.5
./Goat -gopath tmp -modulepath etcd/raft \
       -task collect-primitives -metrics -psets gcatch -include-tests \
       go.etcd.io/etcd/raft/v3
```

### Notes

Analyzing code that uses generics is currently not supported.
