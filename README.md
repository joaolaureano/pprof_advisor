# profadvisor

Finds a hot path in **any** Go package that has benchmarks, asks a language model
how to fix it, then measures whether the fix actually worked. It optimizes CPU
time, memory allocation, or contention (block or mutex), chosen per run.

It has no target of its own and knows nothing about the code it is pointed at:
you give it a directory and a package pattern, and everything it reports comes
from the profile and benchmark output of that run.

## Install

```
go build -o profadvisor .
```

Requires a Go toolchain on PATH — the target's benchmarks have to compile — and,
for the `analyze` step only, an API key for a model provider.

## Use

`--dir` is the target repository's root; `--pkg` is a package pattern resolved
inside it.

```
export PROFADVISOR_API_KEY=...

./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --bench . --count 10

# optimize allocation instead of time
./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --profile memory

# optimize for lock contention
./profadvisor run --dir /path/to/your/repo --pkg ./internal/sync/ --profile mutex
```

`run` does capture → extract → analyze → apply → capture and hands you the two
benchmark outputs; `verify` turns those into a verdict. A suggestion is only a
success when that second measurement says so — the model's confidence is not
evidence.

Every command writes JSON to stdout. Add `--format text` to read the same
document yourself:

```
./profadvisor extract cpu.prof --format text
```

## Commands

| Command | What it does |
|---|---|
| `capture` | Runs the benchmark once; keeps the profile and `bench.txt`. |
| `extract` | Ranks hot functions, filters runtime noise, attaches source. |
| `analyze` | Asks a model for a diagnosis and a diff. A hypothesis, not a result. |
| `apply` | Applies the diff on a branch. Refuses a dirty tree. |
| `verify` | The only command that reaches a verdict, from two `bench.txt` files. |
| `run` | The five stages in order. Stops before judging. |
| `escape` | Reports the compiler's escape analysis. No benchmark, no API key. |
| `benchgen` | Generates fuzz tests and benchmarks offline from a frozen corpus. |

[AGENTS.md](AGENTS.md) is the full reference: every flag, the I/O contract, the
schema versions, and what each verdict means. Read it before scripting against
this tool.

## Escape analysis

```
./profadvisor escape --dir /path/to/your/repo
```

A separate question that needs no benchmark, no profile, and no API key: what
the compiler concluded about which values are heap-allocated.

**The compiler is the source of truth, and an escape is not a defect.** A heap
allocation on a path that runs once costs nothing measurable. This command
reports evidence; deciding that code should change still requires the loop
above. There is no severity, no ranking, and no suggestion in the output,
deliberately.

## Generating benchmarks

`benchgen` turns a frozen directory of Go fuzz corpus files into a self-contained
`_test.go` file and a manifest, using native Go fuzzing and fixed templates. No
model, API key, or benchmark execution is involved.

```sh
./profadvisor benchgen --dir /path/to/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write
```

The corpus format, the accepted parameter types, interface selection, and the
replay steps are in [AGENTS.md](AGENTS.md).

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

Every word the tool sends lives in `internal/prompt/prompts.json`, embedded at
build time. Copy it, edit it, and pass `--prompts <file>` to try different
wording without rebuilding.

## Scope

CPU, allocation, block-contention, and mutex-contention profiles over
`go test -bench`. Trace profiles are not covered. A target that waits on I/O,
network, or databases will get a confident answer that means nothing.

A memory run optimizes `B/op` (or `allocs/op`) and keeps `ns/op` as a guard, so a
patch that saves bytes by spending time is rejected rather than celebrated.
Block and mutex runs optimize `ns/op`; recording every contention event adds
overhead, so absolute values from a contention capture are not comparable to a
clean run — but baseline and after use identical flags, so the verdict stays
valid.

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
| `internal/verify/` | benchfmt + benchmath. The only place a verdict is reached. |
| `internal/benchgen/` | Generates fuzz tests and benchmarks from a frozen corpus. |
| `internal/render/` | Documents and values as readable text, for `--format text` and for prompts. |
| `internal/escape/` | Parses the compiler's escape diagnostics. The only place their wording lives. |
| `internal/toolchain/` | Finds the Go toolchain that will compile the target, and runs it. |
| `internal/proc/` | Process-group handling for the packages that shell out to `go`. |
| `internal/pipeline/` | Runs the five stages in order. Produces artifacts; judges nothing. |
| `internal/schema/` | The JSON contract between subcommands. |
| `internal/fixture/` | Loads the recorded profiles under `testdata/` for tests. |
| `testdata/fixture/` | A small Go module whose benchmarks produce those profiles. |
| `testdata/escape/` | A corpus module and the recorded compiler output the parser is tested on. |
| `testdata/prompts/` | Golden files pinning the exact bytes sent for each objective. |

`internal/*` never prints and never exits; it returns values and errors. That is
what makes each step testable without a process.

## Testing

```
go test ./...
```

The suite is self-contained. `testdata/fixture/` is a throwaway module with a
deliberately slow path matcher, and the committed profiles and benchmark output
were recorded from its benchmarks. The end-to-end tests run the real loop against
that module and against a temporary git repository the test creates, so nothing
outside this checkout is needed and no API key is used.

`testdata/escape/` works the same way for the escape parser. The parser tests run
against a recording and need no toolchain; one further test runs the real
compiler and compares. After a Go upgrade, that comparison is what tells you the
wording moved — re-record with `go test ./internal/escape -update` and read the
diff.

The prompt catalog is pinned the same way: `go test ./internal/analyze -update`
regenerates `testdata/prompts/`, and the diff is the review.
