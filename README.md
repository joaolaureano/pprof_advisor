# profadvisor

Finds a hot-path in **any** Go package that has benchmarks, asks a language model
how to fix it, and then measures whether the fix actually worked. It optimizes CPU time
or memory allocation, chosen per run.

It also answers a second, narrower question that needs no benchmark: what the Go
compiler's escape analysis concluded about a package. See
[Escape analysis](#escape-analysis).

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
you are working on a different project. `run` does capture → extract → analyze
→ apply → capture and hands you the two benchmark outputs; `verify` turns those
into a verdict. A suggestion is only a success when that second measurement says
so — the model's confidence is not evidence.

Every command writes JSON to stdout. Add `--format text` to read the same
document yourself instead:

```
./profadvisor extract cpu.prof --format text
```

See [AGENTS.md](AGENTS.md) for the full command reference and the I/O contract.
That file is what both humans and agents should read first.

## Generate benchmarks and fuzz tests offline

`benchgen` turns a frozen directory of Go fuzz corpus files into a self-contained
`_test.go` file and a manifest. It uses native Go fuzzing and fixed templates;
no model, API key, or benchmark execution is involved in generation.

Each corpus file must contain one value matching the function's argument:

```text
go test fuzz v1
string("example")
```

For a `[]byte` argument, use `[]byte("example\\x00\\xff")` instead of
`string("example")`. Pass the directory containing these files explicitly:

```sh
./profadvisor benchgen --dir /path/to/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write
```

One execution handles one package-level function, including unexported functions.
It must be non-generic, non-variadic, and accept exactly `string` or `[]byte`.
Return values, including errors, are discarded. Methods and custom setup are
unsupported, as are packages using cgo. The target must use a Go 1.24+
toolchain for `b.Loop()`.
The function must be deterministic, independent of external state, and must
neither modify nor retain its argument. These are caller obligations; generation
cannot prove them.

The generated `FuzzProfadvisor_parse` embeds each unique seed with `f.Add`.
It detects panics; it defines no additional correctness property. The generated
`BenchmarkProfadvisor_parse` has one sub-benchmark per seed, named by its SHA256.
Inputs are prepared before `b.Loop()`, allocations are reported, and the measured
loop calls the function directly. No fuzzing or random input generation occurs
inside the benchmark.

`--out` receives the code and manifest. `--write` additionally installs the same
code in the target package. Existing files and conflicting symbols are refused.
The JSON report and manifest use their own `schema_version: 1`; the report says
`generated: true` and `validated: false` because the target has not been executed.
Use `--format text` for a readable report. Diagnostics use stderr and exits are
0 for success or 1 for failure, as with the other commands.

Replay seeds before measuring, from the target repository:

```sh
go test ./internal/parser -run '^FuzzProfadvisor_parse$'
go test ./internal/parser -run '^$' -bench '^BenchmarkProfadvisor_parse$' -benchmem -count 10
# Optional exploration, separate from measurement:
go test ./internal/parser -run '^$' -fuzz '^FuzzProfadvisor_parse$' -fuzztime 30s
```

You may explicitly import a selected directory from Go's fuzz cache as `--corpus`.
Coverage discoveries reside in that cache; `testdata/fuzz` may also contain inputs
that caused failures. Inspect and replay imported seeds. Each generation freezes
the selected corpus: it does not follow the cache or choose inputs by speed.
Keep identical generated code and corpus hashes for baseline and after runs.
Version the test and manifest before using `run`, which requires a clean git
tree. `benchgen` is not automatically invoked by `run`; measurement and `verify`
remain separate steps.

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
sentence is visible rather than silently dropped. One finding looks like this:

```json
{
  "kind": "moved_to_heap",
  "package": "example.com/svc/pkg/wal",
  "file": "pkg/wal/wal.go", "line": 203, "column": 6,
  "subject": "cabecalho",
  "function": "(*Log).Append",
  "flow": [
    { "text": "{heap} ← &cabecalho:" },
    { "text": "cabecalho", "reason": "address-of",
      "file": "pkg/wal/wal.go", "line": 207, "column": 36 },
    { "text": "(*bufio.Writer).Write(l.buf, cabecalho[:])", "reason": "call parameter",
      "file": "pkg/wal/wal.go", "line": 207, "column": 26 }
  ],
  "evidence": "pkg/wal/wal.go:203:6: moved to heap: cabecalho"
}
```

That is a real record with one flow hop elided and the module path replaced; the
`flow` is the compiler's own account of how the value reached the heap, read
bottom-up.

Match on `kind`. `evidence` is there so the record outlives any future change to
how this is modelled, not so consumers can grep it.

### What "covers the vocabulary" means here

Passing on a corpus only proves the parser handles what that corpus happened to
provoke, which is a weak guarantee: three conclusions are gated behind a compiler
debug flag, and the warning the compiler prints when it abandons a flow at an
assignment cycle appeared in neither the corpus nor a real nine-package service
with 5868 diagnostic lines.

So the inventory is transcribed from the format strings in
`cmd/compile/internal/escape` and checked as a table —
`TestParseCoversEveryCompilerTemplate`, 28 templates, each mapped to the kind it
must produce or to the decision not to report it. Two things are outside it on
purpose: the compiler's two hard errors, which fail the build instead of reaching
a report, and the per-statement tracing, which needs `-m=3` while this tool fixes
`-m=2`.

That inventory is a snapshot of one release. A newer toolchain than the parser
was validated against is reported in `warnings` and still parsed, and anything
genuinely new lands in `unrecognized` — the guarantee is that a gap announces
itself, not that gaps cannot happen.

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
| `internal/verify/` | benchfmt + benchmath. The only place a verdict is reached. |
| `internal/render/` | Documents and values as readable text, for `--format text` and for prompts. |
| `internal/escape/` | Parses the compiler's escape diagnostics. The only place their wording lives. |
| `internal/toolchain/` | Finds the Go toolchain that will compile the target, and runs it. |
| `internal/proc/` | Process-group handling for the packages that shell out to `go`. |
| `internal/pipeline/` | Runs the five stages in order. Produces artifacts; judges nothing. |
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

Those recordings prove the parser handles real output; the template table
described above proves it handles output the fixtures never produced. Both are
needed, and neither substitutes for the other.
