# profadvisor

Finds a hot-path in **any** Go package that has benchmarks, asks Claude how to
fix it, and then measures whether the fix actually worked. It optimizes CPU time
or memory allocation, chosen per run.

profadvisor is a standalone tool. It has no target of its own and knows nothing
about the code it is pointed at: you give it a directory and a package pattern,
and everything it reports comes from the profile and the benchmark output of
that run.

## Install

```
go build -o profadvisor .
```

Requires a Go toolchain on PATH (the target's benchmarks have to compile) and,
for the `analyze` step only, `ANTHROPIC_API_KEY`.

## Use

Point it at the repository you want to make faster. `--dir` is that
repository's root and `--pkg` is a package pattern interpreted inside it:

```
export ANTHROPIC_API_KEY=...

./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --bench . --count 10

# optimize allocation instead of time
./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --profile memory
```

Nothing about the target is assumed or configured anywhere: change `--dir` and
you are working on a different project. The loop is capture → extract → analyze
→ apply → capture → verify, and a suggestion is only a success when the second
measurement says so; the model's confidence is not evidence.

See [AGENTS.md](AGENTS.md) for the full command reference and the I/O contract.
That file is what both humans and agents should read first.

## Scope

CPU and allocation profiles over `go test -bench`. Block, mutex, and trace
profiles are not covered, and a target that waits on I/O will get a confident
answer that means nothing.

A memory run optimizes `B/op` (or `allocs/op`) and keeps `ns/op` as a guard, so
a patch that saves bytes by spending time is rejected rather than celebrated.

## Layout

| Path | What it holds |
|---|---|
| `cmd/` | Flag parsing, exit codes, JSON on stdout. No logic. |
| `internal/measurement/` | What a run optimizes: sample type, attribution, metric roles. |
| `internal/benchmark/` | One `go test -bench` invocation. Process handling only. |
| `internal/capture/` | Keeps the profile and `bench.txt` from one run. |
| `internal/extract/` | Ranks hot functions, filters runtime noise, attaches source. |
| `internal/analyze/` | Builds the prompt and calls the Anthropic API. |
| `internal/apply/` | Applies the diff on a branch. Refuses a dirty tree; rolls back. |
| `internal/verify/` | benchfmt + benchmath. Returns MELHOROU / SEM DIFERENÇA / PIOROU. |
| `internal/pipeline/` | Runs the five in order and decides what the result means. |
| `internal/schema/` | The JSON contract between subcommands. |
| `internal/fixture/` | Loads the recorded profiles under `testdata/` for tests. |
| `testdata/fixture/` | A small Go module whose benchmarks produce those profiles. |

`internal/*` never prints and never exits; it returns values and errors. That is
what makes each step testable without a process.

## Testing

```
go test ./...
```

The suite is self-contained: `testdata/fixture/` is a throwaway module with a
deliberately slow path matcher, and the committed profiles and benchmark output
were recorded from its benchmarks. The end-to-end tests run the real loop
against that module and against a temporary git repository the test creates,
so nothing outside this checkout is needed and no API key is used.
