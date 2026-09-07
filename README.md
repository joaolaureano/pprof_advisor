# profadvisor

Finds a hot path in **any** Go package that has benchmarks, asks a language model
how to fix it, then measures whether the fix actually worked. It has no target of
its own and knows nothing about the code it is pointed at: you give it a
directory and a package pattern, and everything it reports comes from the profile
and benchmark output of that run.

## Scope

It reads CPU, allocation, block-contention and mutex-contention profiles over
`go test -bench`. Trace profiles are not covered, and it captures no I/O,
network or database latency — a target whose cost is there is profiled as
though it were idle.

`analyze` produces a hypothesis. `verify` compares two benchmark outputs and is
the only command that emits a verdict.

## Install

```
go build -o profadvisor .
```

Requires a Go toolchain on PATH — the target's benchmarks have to compile — and,
for the `analyze` step only, an API key for a model provider.

## Use

Point it at a repository with `--dir` and a package inside it with `--pkg`.

```
export PROFADVISOR_API_KEY=...

./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --bench . --count 10
```

### The loop

`run` does capture → extract → analyze → apply → capture: it benchmarks the
package, finds the hot functions, asks a model for a diff, applies it on a
branch, and benchmarks again. It stops there and hands you the two benchmark
outputs. Turning them into a verdict is a separate step:

```
./profadvisor verify --baseline <baseline bench.txt> --after <after bench.txt>
```

That prints `MELHOROU`, `SEM DIFERENÇA`, or `PIOROU`, with a percentage delta, a
p-value and the sample counts. Nothing before this step measures anything; the
model's `confidence` field is its own estimate. All three verdicts exit 0.

### Choosing what to optimize

One flag selects the objective, and it reaches every stage: which pprof sample
type is read, how the prompt frames the task, and which metric decides the
verdict.

```
# CPU time — the default, optimizes ns/op
./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/

# allocation — optimizes B/op, or allocs/op with --unit
./profadvisor run --dir /path/to/your/repo --pkg ./internal/parser/ --profile memory

# lock or channel contention
./profadvisor run --dir /path/to/your/repo --pkg ./internal/sync/ --profile mutex
./profadvisor run --dir /path/to/your/repo --pkg ./internal/sync/ --profile block
```

A memory run carries `ns/op` as a guard: it can turn the roll-up into `PIOROU`
but never into `MELHOROU`. Contention runs are judged on `ns/op` too. Recording
every contention event adds overhead, so absolute numbers from a contention
capture are not comparable to a clean run; baseline and after are captured with
identical flags.

### Escape analysis, without a benchmark

A narrower question that needs no benchmark, no profile, and no API key: what the
compiler concluded about which values are heap-allocated.

```
./profadvisor escape --dir /path/to/your/repo
```

The conclusions are the compiler's, reported as normalized JSON. There is no
severity, no ranking, and no suggestion in the output. A heap allocation on a
path that runs once costs nothing measurable, and this command measures
nothing — that is what the loop above does.

### Reading the output

Every command writes JSON to stdout and diagnostics to stderr. `--format text`
renders the same document for a person:

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
schema versions, and what each verdict means.

## Generating benchmarks

`benchgen` turns a frozen directory of Go fuzz corpus files into a self-contained
`_test.go` file and a manifest, using native Go fuzzing and fixed templates. No
model, API key, or benchmark execution is involved.

```sh
./profadvisor benchgen --dir /path/to/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write
```

The generated fuzz target checks for panics and nothing else: it passes
unchanged when the function is edited to return a wrong answer.

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
build time. Copy it, edit it, and pass `--prompts <file>` to try different wording
without rebuilding.

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
