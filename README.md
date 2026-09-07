# profadvisor

Finds a hot-path in **any** Go package that has benchmarks, asks a language model
how to fix it, and then measures whether the fix actually worked. It optimizes CPU time
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
for the `analyze` step only, an API key for a model provider.

## Use

Point it at the repository you want to make faster. `--dir` is that
repository's root and `--pkg` is a package pattern interpreted inside it:

```
export PROFADVISOR_API_KEY=...

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

## Providers

The tool has no built-in vendor. `internal/analyze` builds a request in neutral
types and one adapter under `internal/llm/` speaks the wire format:

```
--provider anthropic     # default
--provider openai
--base-url http://localhost:8080   # any endpoint speaking that provider's format
--model <id>             # default: whatever the provider picks
```

Each can also come from the environment: `PROFADVISOR_PROVIDER`,
`PROFADVISOR_MODEL`, `PROFADVISOR_BASE_URL`, `PROFADVISOR_API_KEY`. The key falls
back to the provider's own conventional variable if the neutral one is unset.

Adding a provider is one file: implement `llm.Client`, call `llm.Register` from
`init()`, and blank-import it in `internal/llm/providers`.

## Prompts

Every word the tool sends lives in `internal/prompt/prompts.json`, embedded at
build time. Entries carry their text and the placeholders it declares, and the
two are checked against each other at load, so a mistyped `{unit}` fails at
startup rather than reaching a model after a benchmark has already been paid
for.

To try different wording without rebuilding, copy the file, edit it, and pass
`--prompts <file>`. The exact bytes sent for each objective are pinned by
golden files under `testdata/prompts/`; `go test ./internal/analyze -update`
regenerates them, and the diff is the review.

## Escape analysis

A separate question, answered by a separate command:

```
./profadvisor escape --dir /path/to/your/repo
```

This one needs no benchmark, no profile, and no API key. It compiles the target
with the compiler's own escape analysis turned on and reports what the compiler
concluded — which values are heap-allocated, which stay on the stack, which
parameters outlive their call.

**The compiler is the source of truth and an escape is not a defect.** A heap
allocation on a path that runs once costs nothing measurable. This command
reports evidence; deciding that code should change still requires the loop
above. There is no severity, no ranking, and no suggestion anywhere in the
output, deliberately.

Compiler diagnostic text is prose, not an API — it has been reworded across
releases and will be again. So it is parsed once, at a boundary, into a stable
vocabulary: a finding carries a `kind` such as `moved_to_heap` or
`leaking_param_result`, and the compiler's original line beside it as
`evidence`. Nothing downstream ever matches on the wording. The exact toolchain
that reached the conclusion is recorded too, because the same source can
legitimately get a different answer from a different compiler.

A diagnostic the parser does not recognize is never guessed at. It is preserved
verbatim under `unrecognized` and counted, so a toolchain that has learned a new
sentence is visible rather than silently dropped.

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
| `internal/analyze/` | Builds the request and reads the answer. Names no vendor. |
| `internal/prompt/` | Every prompt the tool sends, as one validated JSON catalog. |
| `internal/llm/` | Neutral model client; one adapter subpackage per provider. |
| `internal/apply/` | Applies the diff on a branch. Refuses a dirty tree; rolls back. |
| `internal/verify/` | benchfmt + benchmath. Returns MELHOROU / SEM DIFERENÇA / PIOROU. |
| `internal/escape/` | Parses the compiler's escape diagnostics. The only place their wording lives. |
| `internal/toolchain/` | Finds the Go toolchain that will compile the target, and runs it. |
| `internal/proc/` | Process-group handling for the packages that shell out to `go`. |
| `internal/pipeline/` | Runs the five in order and decides what the result means. |
| `internal/schema/` | The JSON contract between subcommands. |
| `internal/fixture/` | Loads the recorded profiles under `testdata/` for tests. |
| `testdata/fixture/` | A small Go module whose benchmarks produce those profiles. |
| `testdata/escape/` | A corpus module and the recorded compiler output the parser is tested on. |

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

`testdata/escape/` works the same way for the escape parser: a corpus module of
small functions, each written to make the compiler reach one particular
conclusion, plus that compiler's recorded output. The parser tests run against
the recording and need no toolchain; one further test runs the real compiler and
compares. After a Go upgrade, that comparison is what tells you the wording
moved — re-record with `go test ./internal/escape -update` and read the diff.
