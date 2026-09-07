# profadvisor

A CLI that finds a hot-path in a Go benchmark, asks a language model how to fix
it, and
then measures whether the fix actually worked. It optimizes CPU time, memory
allocation, or contention (block or mutex), chosen per run.

It answers questions a benchmark can settle: why a Go package is slow or
allocation-heavy, and whether a given change improves it. It is not a profiler
viewer and not a general optimizer.

## What it is pointed at

profadvisor is target-agnostic and stateless about targets. It has no
configured project, no default repository, and no knowledge of any codebase
other than the one named on the command line:

- `--dir` is the root of the repository under test. It defaults to the current
  directory, so pass it explicitly whenever the target is elsewhere.
- `--pkg` is a `go test` package pattern resolved **inside `--dir`**, not
  inside this tool's checkout.
- `--profile` and `--unit` choose what the run optimizes, and default to CPU
  time. See "Choosing the objective" below.

Everything reported — hotspots, module name, source excerpts, the verdict —
comes from that run. If you are reading this file as a reference while working
on some other repository, the target is that repository: pass its path as
`--dir`. Nothing in this document names a project to run against, and the
examples below use placeholder paths that you are expected to replace.

## When to use it

Use it when **all** of these hold:

- The target is a Go package with benchmarks (`go test -bench`), or you can write them.
- The work is CPU-bound or allocation-bound — algorithms, parsing, routing,
  serialization, business logic.
- The question is "what is expensive and does this change help", not "is this
  design good".

`escape` is the exception to all of the above and answers a different question:
*is this value heap-allocated at all*. It needs no benchmark, so it is the one
thing here that is worth running on a package with none. It reports what the
compiler concluded and does not claim any of it costs anything — see below.

Do **not** use it when:

- The suspected cost is I/O, network, or database latency. Those need profiles
  this tool does not capture, and it will produce a confidently wrong answer.
  Lock and channel contention are in scope via `--profile block` and
  `--profile mutex`.
- There is no benchmark and the code cannot be exercised deterministically.
  Without a benchmark there is nothing to compare against, and the verdict step —
  the only step that measures anything — cannot run.

## Choosing the objective

The objective is set once and reaches every stage: it selects which pprof sample
type is read, which frame a cost is charged to, how the prompt frames the task,
and which metric decides the verdict. That is why it is one flag and not four.

Four kinds are supported:

- `--profile cpu` (the default) optimizes `ns/op` from a CPU profile.
- `--profile memory` optimizes `B/op` from an allocation profile, or `allocs/op`
  with `--unit allocs/op`. The profile can be left off when the unit implies it:
  `--unit B/op` alone means a memory run.
- `--profile block` optimizes `ns/op` from a block-contention profile, reporting
  time goroutines spend waiting on channels and semaphores.
- `--profile mutex` optimizes `ns/op` from a mutex-contention profile, reporting
  time goroutines spend waiting on mutexes.

**Block and mutex profiles are judged on `ns/op` like CPU runs**, not on a
contention metric. `go test -bench` reports no contention measurement, and a
profile holds one sample per capture with no statistic to compare. The profile
changes what evidence the model is shown about where time goes; it does not
change what decides the verdict. Contention that costs no wall-clock time is not
worth a patch.

**A memory run keeps `ns/op` as a guard.** Trading time for memory is easy and
usually not what was asked for, so a patch that significantly slows the
benchmark is `PIOROU` even when it allocates less. The two memory units do not
guard each other: fewer bytes in more allocations is a legitimate outcome, so
the unit you did not choose is reported and votes on nothing.

**Attribution to the focus frame:** Both allocation and contention profiles are
attributed to the innermost frame inside the focus package, not to the leaf. The
leaf frame of every allocation sample is `runtime.mallocgc`, and the leaf of a
contention sample is `sync.(*Mutex).Lock` or `runtime.chanrecv` — standard
library or runtime code that nobody can edit. Charging cost to the leaf would
rank the allocator or synchronization primitive first and bury the code under
test. Instead each sample is charged to the call that asked for the memory or
caused the wait, which is the line a patch can change.

### Caveats on contention profiles

1. **Delay is time blocked, summed across goroutines.** Block and mutex profiles
   measure the sum of delays on all goroutines combined. In a contention trace
   from a worker pool, each blocked worker contributes its delay, so the total
   routinely exceeds the benchmark's wall-clock time.

2. **Focus inference can miss in concurrent code.** `extract` identifies the
   focus package from the `Benchmark*` frame in the profile. In a worker-pool
   benchmark the goroutine that blocks was created by a worker and does not
   descend from `Benchmark*`, so inference falls back to "busiest non-stdlib
   module". When that picks wrong, use `--focus` to override.

3. **Profiling overhead inflates the numbers.** Turning on block profiling with
   `-blockprofilerate=1` records every blocking event, which costs time and
   inflates `ns/op`. Baseline and after are captured with identical flags. The
   absolute numbers from a contention capture are not comparable to a clean run.
   `--rate` trades detail for overhead.

## Requirements

- A Go toolchain on PATH. The target's benchmark has to compile, so this is
  structural; nothing else needs installing.
- The target repository is a git repository if you intend to use `apply` or
  `run`; the diff is applied on a branch there, and a dirty tree is refused.
- An API key for the selected model provider: `PROFADVISOR_API_KEY`, or that
  provider's own conventional variable. Needed by `analyze` and `run` only;
  `capture`, `extract`, `verify`, and `escape` work offline.
- `escape` additionally needs nothing else: no benchmark, no profile, no git
  repository. It only needs the target to compile.

## Providers and prompts

No vendor is built in. `--provider` (or `PROFADVISOR_PROVIDER`) selects an
adapter under `internal/llm/`; `anthropic` is the default and `openai` is the
other one shipped. `--model`, `--base-url` and the API key come from flags or
from `PROFADVISOR_MODEL`, `PROFADVISOR_BASE_URL` and `PROFADVISOR_API_KEY`, with
the key falling back to the provider's own conventional variable. `--base-url`
points at any endpoint speaking the selected provider's wire format, including a
local one.

Every word sent lives in `internal/prompt/prompts.json`, embedded at build time.
Entries carry their text and the placeholders they declare, and the two are
checked against each other at load, so a mistyped `{unit}` fails at startup
rather than after a benchmark has already been paid for. `--prompts <file>`
overrides the catalog without rebuilding. The exact bytes sent for each
objective are pinned by golden files under `testdata/prompts/`.

## I/O contract

Every subcommand follows the same rules:

- **stdout** carries the result, and carries nothing else. `--format` chooses
  the rendering: `json` (the default, indented) or `text`. Both render the same
  document; `text` is the human-readable form.
- **stderr** carries every diagnostic, progress line, and error message.
- **exit 0** means the tool worked and the document on stdout is complete.
  **exit 1** means it failed; the document is absent or partial — do not parse
  stdout after a non-zero exit.

There is no exit code 2. It used to mean "the tool worked and the answer was
bad", which put a measured regression into the process contract; a verdict is
now a field in a document and nothing else. See "Reporting and judging" below.

Each document carries `schema_version`, currently **3**, with one exception:
`capture` writes no version field. Its result is an artifact index — the two
paths plus the `go` invocation — so do not branch on a version when reading it.
The version changes when the shape of the document changes.

`escape` is the exception, and deliberately so: its report carries
`schema_version` **1** from a separate constant. The capture → extract → analyze
→ apply → verify documents are links in one chain and move together; the escape
report is not in that chain, and its reason to change is the compiler's
diagnostic vocabulary. Sharing one number would bump five documents every time
the toolchain rewords a line.

## Reporting and judging

There are two kinds of command:

**Reporters** — `capture`, `extract`, `analyze`, `apply`, `escape`, `run`. They
produce a document describing what they found or did. None of them decides
whether anything is good. `escape` is the clearest case: a heap allocation is
evidence, not a defect.

**One judge** — `verify`. It is the only command that reaches a verdict, and it
does so from two benchmark outputs, not from any other document this tool
produces. A p-value needs N samples of a metric; a profile has none, and a
diagnosis is a hypothesis.

The verdict is not an opinion in the loose sense. The statistics are mechanical
— `benchmath.AssumeNothing`, a non-parametric comparison — and the `p_value`,
`delta_pct` and sample counts are all in the document, and the roll-up is
derived from them. Two things are policy: the significance level (`--alpha`,
default 0.05) and which metric plays which `role`, which follows from
`--profile`/`--unit`. The roles appear in the document; the alpha that produced
them does not.

`run` therefore stops before judging. It captures, extracts, diagnoses,
applies, and re-captures, then hands you the two `bench.txt` paths and the
`verify` invocation that turns them into a verdict.

`capture`, `extract`, `analyze`, `verify` and `run` also carry a `measurement`
object — profile, unit, pprof sample type, sample unit, and attribution rule. It
is top-level in all of them except `extract`, which nests it under `profile`.
`apply` has none: it moves a diff onto a branch and reads no metric. Cost fields (`total`, `analyzed`,
`flat`, `cum`, `line_costs`) are plain numbers in `measurement.sample_unit`:
nanoseconds for a CPU run, bytes or object counts for a memory one. **Do not
assume nanoseconds.** Version 1 named these fields `*_nanos` and had no
`measurement`; that is the whole difference, and why the version moved.

## Commands

### `profadvisor benchgen --dir <repo> --pkg <package> --func <name> --corpus <directory> --out <directory> [--write] [--impl Interface=Type ...]`

Generates offline same-package fuzz tests and benchmarks from a frozen Go fuzz
v1 corpus. Accepts one non-generic, non-variadic package-level function with one
or more native Go fuzz arguments: `string`, `[]byte`, `bool`, every built-in signed or
unsigned integer type, `rune`, `byte`, `float32`, or `float64`, or local structs
composed recursively from those types, or named interfaces, including unexported functions.
Struct fields are flattened recursively in declaration order and reconstructed with keyed
literals. Defined scalar types, pointers, maps, arbitrary slices, external structs, blank
fields, structs without any supported leaf field, anonymous interfaces, `any`/`interface{}`
(zero-method interfaces), and cycles are not supported. Corpus values use their
exact explicit conversion, one line per flattened native value in signature order,
for example `int64(-42)` or `bool(true)`. A corpus file is one complete
argument tuple. `uintptr` is not a native Go fuzz type and is refused.
Requires a Go 1.24+ target toolchain. Returns are discarded; fuzz replay detects panics
without inventing correctness properties or treating returned errors as failures.
Generation does not check determinism, external state, or whether the function
mutates or retains its arguments.

**Interface parameters**: When a parameter has an interface type, `benchgen` selects a
concrete local type that implements it. The interface may be declared anywhere — in
the target package or imported from the standard library — but the concrete
implementation must be declared in the target package, because the generated file
adds no imports. If exactly one local type implements the interface and can be flattened into native
fuzz types, it is used: value receiver produces `T{...}`, pointer receiver produces
`&T{...}`. If multiple local types implement the interface after filtering out
non-flattenable types, generation fails with an error listing them; use `--impl
Interface=Type` to pick one explicitly. Explicit `--impl` pairs are never ambiguous
and override discovery. Both interface spellings are accepted: `--impl Reader=Type`
for a local `Reader`, and `--impl io.Reader=Type` for an imported interface.
The benchmark measures the chosen implementation, not "the interface": cost behind
an interface call is entirely the implementation's. If the choice changes between
two measurements, `verify` is comparing different programs. The chosen type is
recorded in the manifest's `implementations` field.

The output directory receives a record — the generated code plus a build
constraint that excludes it from compilation — alongside the manifest. Because
the constraint keeps the file out of every build, committing it inside the
target module produces no duplicate-symbol or orphan-package error.

`--write` additionally installs the live test file into the target package, without
any build constraint. That is the copy that runs when you execute the tests.

Generation refuses to proceed when the target package already declares
`FuzzProfadvisor_<name>` or `BenchmarkProfadvisor_<name>`, or when the file it
would install already exists. **This check runs even without `--write`**, so a
second `--out`-only run against a package that still holds a previously
installed copy fails rather than rewriting the record. Deleting the installed
file clears the conflict.

This reporter has its own schema version **4** and no measurement object.
`generated: true` with `validated: false` means generation succeeded, not that
the seeds passed: generation never executes the target and needs no API key.
Schema version 4 added support for interface parameters and the `implementations`
field in the manifest, which records the chosen concrete type for each interface
as `"Interface=Type"` strings.

Replay the generated `FuzzProfadvisor_<name>` seeds before measuring
`BenchmarkProfadvisor_<name>`. Exploration via `go test -fuzz` is a separate user
step. Cache directories are imported only when named explicitly. A corpus that
differs between baseline and after produces two measurements of different
inputs. `run` requires a clean tree, so the generated tests and manifest have to
be committed first. There is no automatic
integration with `run`.

**Corpus format.** Each file holds one complete argument tuple: one value per
flattened parameter, in the function's parameter order, using the value's exact
explicit conversion. A function with one `string` parameter therefore has one
line after the header:

```text
go test fuzz v1
string("example")
```

Use `[]byte("example\x00\xff")` for a `[]byte` parameter, and `int64(-42)`,
`bool(true)` or `float64(1.5)` for scalars. Struct arguments contribute one line
per supported leaf field, in declaration order, recursively — so a function
taking `string`, `int64` and `bool` has three lines. The native special-float
forms `NaN`, `+Inf`, `-Inf`, and `math.Float32frombits` /
`math.Float64frombits` are accepted.

**A complete example**, from the target repository:

```sh
profadvisor benchgen --dir /path/to/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write

go test ./internal/parser -run '^FuzzProfadvisor_parse$'
go test ./internal/parser -run '^$' -bench '^BenchmarkProfadvisor_parse$' -benchmem -count 10
# Optional exploration, separate from measurement:
go test ./internal/parser -run '^$' -fuzz '^FuzzProfadvisor_parse$' -fuzztime 30s
```

The generated `FuzzProfadvisor_parse` embeds each unique tuple with `f.Add` and
detects panics; it defines no additional correctness property. The generated
`BenchmarkProfadvisor_parse` has one sub-benchmark per seed, named by its
SHA256. Inputs are prepared before `b.Loop()`, allocations are reported, and the
measured loop calls the function directly. No fuzzing or random input generation
occurs inside the benchmark.

You may explicitly import a selected directory from Go's fuzz cache as
`--corpus`; coverage discoveries live in that cache, and `testdata/fuzz` may
also hold inputs that caused failures. Each generation freezes the selected
corpus: it does not follow the cache or choose inputs by speed. Keep identical
generated code and corpus hashes for baseline and after.

### `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory|block|mutex] [--bench <regexp>] [--count N] [--rate N]`

Runs the benchmark once and writes a timestamped directory under
`./profadvisor-out/` containing the profile and `bench.txt`.

The profile written depends on `--profile`:
- `cpu.prof` for a CPU profile (default)
- `mem.prof` for a memory profile
- `block.prof` for a block-contention profile
- `mutex.prof` for a mutex-contention profile

Both artifacts come from the same run, deliberately: the profile feeds the
diagnosis and `bench.txt` feeds the verdict, and if they came from different runs
they would describe different programs.

`-benchmem` is always on, whatever the objective, because `verify` needs
`ns/op`, `B/op` and `allocs/op` from one output in order to guard one against
another.

`--count` defaults to 10. Lowering it makes `verify` less able to distinguish a
real change from noise; a single sample has more than once suggested a
regression that turned out to be a double-digit gain.

`--rate` controls sampling detail for contention profiles (block and mutex). It
is `-blockprofilerate` for a block run and `-mutexprofilefraction` for a mutex
run, and defaults to 1 (record every event). It is ignored for CPU and memory
profiles. Recording every blocking event adds overhead that inflates `ns/op`, so
the absolute numbers from a contention capture are not comparable to a clean run
— but baseline and after are captured with identical flags, so the verdict stays
valid.

```
profadvisor capture --dir /path/to/target --pkg ./internal/parser/ --bench '^BenchmarkParse$'
profadvisor capture --dir /path/to/target --pkg ./internal/sync/ --profile mutex --rate 10
```

### `profadvisor extract <profile> [--profile cpu|memory|block|mutex]`

Ranks the hottest functions by self cost and attaches the surrounding source
with per-line cost. `--profile` must match the profile that was captured;
pointing a CPU objective at `mem.prof` or `block.prof` is an error rather than
an empty result.

Runtime and standard-library frames are filtered out of the ranking. This is not
cosmetic: in a short benchmark, idle netpoller threads alone can account for more
than half the samples, and an unfiltered top-N reports `runtime.kevent` as the
hot path while the code under test sits below the fold. Percentages are
renormalized over what survives, and both totals are reported.

The focus — which package counts as "the code under test" — is taken from the
package that defines the `Benchmark*` function in the profile. That is the
reliable signal, and picking the busiest package instead gets it wrong whenever
a callee is hotter than its caller: a per-pixel loop can spend more time inside
`image/color` than in the function that runs it, and focusing there would hide
the only code the user can change. `--focus` overrides the inference.

The output also reports **where the filtered cost went**, in `excluded`. This
matters because filtering can remove the explanation along with the noise: a
loop that boxes a value into an interface shows up as one hot function plus a
wall of `runtime.convTnoptr` and `runtime.mallocgc`, and only the second half
says that the fix is about allocation.

That list is ranked by `from_focus` — the cost on paths the code under test
actually reached — and **not** by `cum`, which is the function's share of the
whole profile. The difference decides whether the list is usable. On a short
benchmark, idle netpoller threads give `runtime.kevent` 40% of `cum` while
having nothing to do with your code; ranking by that buries the frames that
explain anything. Both numbers are reported, and the gap between them is itself
informative: a large `cum` with a small `from_focus` is a frame busy on some
other goroutine.

The list also includes frames outside the focus package, not just runtime
noise. A function that spends half its time in `regexp.Compile` looks merely
slow until `excluded` names the callee.

### `profadvisor analyze <extract.json>`

Sends the hotspots and their source to a language model and returns a diagnosis
plus a unified diff.

The provider is chosen with `--provider` (or `PROFADVISOR_PROVIDER`) and the
wording comes from a prompt catalog, overridable with `--prompts`. Nothing here
is tied to one vendor: `internal/analyze` names none, and each provider is one
adapter under `internal/llm/`.

**The output is a hypothesis.** Nothing in it has been measured, and the model's
own `confidence` field is not evidence. A suggestion only counts once `verify`
confirms it. Treat a high-confidence diff that fails verification as a normal
outcome, not a bug.

### `profadvisor apply <diagnosis.json>`

Applies the diff on a new branch, `profadvisor/suggestion-N`, in the target
repository, never on its working branch. `N` is the next free number, so
repeated runs accumulate branches rather than overwriting one. Use `--dir` to
name that repository; it defaults to the current directory.

**It leaves the repository checked out on the suggestion branch.** That differs
from `run`, which returns the repository to the branch it started on.

### `profadvisor verify --baseline <bench.txt> --after <bench.txt> [--unit <unit>]`

Compares two benchmark outputs and returns `MELHOROU`, `SEM DIFERENÇA`, or
`PIOROU`, with the percentage delta and a p-value.

`comparisons` holds one entry per benchmark **and metric**, each tagged with a
`role`:

- `objective` — the metric named by `--unit`. It decides the verdict.
- `guard` — `ns/op` on a memory run. It can veto, never accept.
- `informational` — reported, never voted.

The roll-up is `PIOROU` if the objective or a guard regressed significantly,
otherwise `MELHOROU` if the objective improved, otherwise `SEM DIFERENÇA`.

Note the inputs: this compares **benchmark output**, not profiles. A p-value
needs N samples of the metric, which a profile does not contain — a profile says
where the cost went, not how much of it there was. Capture both with
`--count 10` or higher on each side, and with `-benchmem` so the guard has
something to read.

### `profadvisor escape [--dir <repo>] [--pkg <pattern>] [--timeout <d>]`

Compiles the target with the compiler's escape analysis enabled and reports what
it concluded, as normalized JSON. `--dir` is the target repository root and
defaults to the current directory; `--pkg` may be repeated and defaults to
`./...`. No binary is written into the target.

The report contains evidence and nothing else: there is no severity field, no
ranking, and no suggestion. The conclusions are the compiler's. A heap
allocation on a cold path costs nothing measurable, and the report carries no
measurement — what a finding costs is what the rest of this tool measures.

Each finding carries a `kind` from a fixed vocabulary — `moved_to_heap`,
`escapes_to_heap`, `does_not_escape`, `leaking_param`, `leaking_param_content`,
`leaking_param_result`, `closure_capture`, and a few more — together with
`evidence`, the compiler's own line, verbatim. `kind` is drawn from that fixed
vocabulary and is stable across toolchains. `evidence` is compiler prose, which
has been reworded between releases.

`toolchain` records the compiler that reached these conclusions, down to the
resolved binary path, and is not decoration: escape analysis improves between
releases, and the same source can legitimately get a different answer from a
different version or architecture. A finding quoted without its toolchain is not
reproducible.

`unrecognized` holds any diagnostic line the parser could not classify, kept
verbatim. It is normally empty. A non-empty list means the toolchain is saying
something this build does not understand yet — the findings that *are* reported
remain accurate, but the picture is incomplete, and `analysis.parser_profile`
plus `warnings` say why.

Two conclusions the compiler can reach — `mutates_param` and `calls_param` — are
gated behind a compiler debug flag this command does not pass, so they will not
normally appear.

One finding looks like this:

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

That is a real record with one flow hop elided and the module path replaced. The
`flow` is the compiler's own account of how the value reached the heap, read
bottom-up.

**What "covers the vocabulary" means here.** Passing on a corpus only proves the
parser handles what that corpus happened to provoke. Three conclusions are
gated behind a compiler debug flag, and the warning the
compiler prints when it abandons a flow at an assignment cycle appeared in
neither the corpus nor a real nine-package service with 5868 diagnostic lines.
So the inventory is transcribed from the format strings in
`cmd/compile/internal/escape` and checked as a table —
`TestParseCoversEveryCompilerTemplate`, 28 templates, each mapped to the kind it
must produce or to the decision not to report it. Two things are outside it on
purpose: the compiler's two hard errors, which fail the build instead of
reaching a report, and the per-statement tracing, which needs `-m=3` while this
tool fixes `-m=2`.

That inventory is a snapshot of one release. A newer toolchain than the parser
was validated against is reported in `warnings` and still parsed, and anything
genuinely new lands in `unrecognized` and is counted, so a gap appears in the
document rather than being dropped from it.

Not every finding is in a file you can edit. The compiler re-analyzes inlined
bodies in the context of the package that inlined them, and it analyzes the
wrapper methods it generates itself, so a small number of findings carry a
standard-library path or the literal file `<autogenerated>` while their
`package` field names *your* package. That attribution is the compiler's, not a
bug, and the findings are reported rather than dropped — but filter on `file`
before treating a finding as something to act on. On a nine-package service this
was four findings out of 683.

### `profadvisor run`

Runs capture → extract → analyze → apply → re-capture in order, and writes the
full record: every intermediate artifact, so a disappointing result can be
investigated without repeating the work. Use the individual commands when you
want to inspect or edit anything in between — in particular, reading the diff
before applying it is usually worth the extra step.

**It does not verify.** The last progress line names the command that does:

```
profadvisor verify --baseline <baseline bench.txt> --after <after bench.txt>
```

Both paths are in the record, under `baseline.bench_path` and
`after.bench_path`. Running that is what turns a run into an answer.

The suggestion branch is kept whatever happens, and the repository is left on
the branch you started from.

## What a result means

A `run` on its own means only that the steps completed. It says nothing about
whether the change helped — that is `verify`'s answer, and you have to ask for
it. Once you do:

- `MELHOROU` — the objective improved significantly. This is the only outcome
  that justifies keeping the change.
- `SEM DIFERENÇA` — the change was neutral. Discard the branch; the hypothesis
  was wrong, and knowing that cost one cycle.
- `PIOROU` — the change was harmful. Discard the branch. This happens and is not
  a malfunction. On a memory run, check which metric caused it: a `PIOROU` from
  the `ns/op` guard with the objective improving means the model bought memory
  with time, and is worth re-running with a narrower `--bench`.

All three exit 0. The verdict is a field in the document; a regression is a
result, not a tool failure. Nothing before `verify` measures anything.
