# Escape-analysis fixtures

`corpus/` is a throwaway Go module — its own module, so the parent's `./...`
never sees it — whose functions exist only to make the compiler reach a
particular conclusion. Each is commented with the scenario it exercises.

Two recordings, both real compiler output, neither hand-edited:

| File | Command, run inside `corpus/` |
|---|---|
| `raw-go1.26.1.txt` | `go build -o /dev/null -gcflags=-m=2 ./...` |
| `raw-mutations-go1.26.1.txt` | the same, plus `-d=escapemutationscalls=1` |

The second exists because three of the compiler's conclusions — `mutates param`,
`calls param`, and `does not escape, mutate, or call` — are gated behind
`base.Debug.EscapeMutationsCalls` and cannot be produced by `-m=2` alone. The
`escape` command does not pass that debug flag: it changes what the compiler
reports, and the report should describe an ordinary build. The recording exists
so the parser is still tested against real output for those templates rather
than against lines someone typed from memory, and so that if a future release
turns them on by default they arrive as findings instead of as unrecognized
text.

The toolchain version is in each filename on purpose. Compiler diagnostics are
prose, and a recording without the compiler that produced it is not evidence of
anything. Re-record with `go test ./internal/escape -update` after a toolchain
upgrade, and read the diff — a change here is a change in the thing being
parsed.
