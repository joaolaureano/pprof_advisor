# profadvisor

A CLI that finds a hot-path in a Go benchmark, asks a language model how to fix
it, and
then measures whether the fix actually worked. It optimizes CPU time, memory
allocation, or contention (block or mutex), chosen per run.

Agents should reach for this tool when someone asks *why* Go code is slow or
allocation-heavy, or asks to make a CPU-bound or allocation-bound Go package
faster. It is not a profiler viewer and not a general optimizer: it only answers
questions a benchmark can settle.

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
  the only part that establishes a change was worth making — cannot run.

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
   routinely exceeds the benchmark's wall-clock time. That is not a bug in the
   report.

2. **Focus inference can miss in concurrent code.** `extract` identifies the
   focus package from the `Benchmark*` frame in the profile. In a worker-pool
   benchmark the goroutine that blocks was created by a worker and does not
   descend from `Benchmark*`, so inference falls back to "busiest non-stdlib
   module". When that picks wrong, use `--focus` to override.

3. **Profiling overhead inflates the numbers.** Turning on block profiling with
   `-blockprofilerate=1` records every blocking event, which costs time and
   inflates `ns/op`. Baseline and after are captured with identical flags so the
   `MELHOROU` / `PIOROU` verdict stays valid, but the absolute numbers from a
   contention capture are not comparable to a clean run. The `--rate` flag exists
   to trade detail for overhead.

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

## I/O contract

Every subcommand follows the same rules, and tooling can rely on them:

- **stdout** carries the result, and carries nothing else. `--format` chooses
  the rendering: `json` (the default, indented) or `text`. Agents should leave
  it at `json`; `text` exists so a person can read the same document.
- **stderr** carries every diagnostic, progress line, and error message.
- **exit 0** means the tool worked and the document on stdout is complete.
  **exit 1** means it failed; the document is absent or partial — do not parse
  stdout after a non-zero exit.

There is no exit code 2. It used to mean "the tool worked and the answer was
bad", which put a measured regression into the process contract; a verdict is
now a field in a document and nothing else. See "Reporting and judging" below.

Each document carries `schema_version`, currently **3**. A consumer that does
not recognize the version should stop rather than guess at the shape.

`escape` is the exception, and deliberately so: its report carries
`schema_version` **1** from a separate constant. The capture → extract → analyze
→ apply → verify documents are links in one chain and move together; the escape
report is not in that chain, and its reason to change is the compiler's
diagnostic vocabulary. Sharing one number would bump five documents every time
the toolchain rewords a line.

## Reporting and judging

Two kinds of command, and it is worth knowing which you are holding:

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
`delta_pct` and sample counts are all in the document, so you can ignore the
roll-up and decide for yourself. What *is* a policy choice is exactly two
things, both declared and both adjustable: the significance level (`--alpha`,
default 0.05) and which metric plays which `role`, which follows from
`--profile`/`--unit`.

`run` therefore stops before judging. It captures, extracts, diagnoses,
applies, and re-captures, then hands you the two `bench.txt` paths and the
`verify` invocation that turns them into a verdict. Chaining it yourself is one
line, and it keeps "what happened" separate from "was it worth it".

Every document also carries a `measurement` object — profile, unit, pprof sample
type, sample unit, and attribution rule. Cost fields (`total`, `analyzed`,
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
The caller must ensure determinism, no external state, and no mutation or
retention of inputs.

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
two measurements, `verify` is comparing different programs and the artifact must say so.

The output directory receives a record — the generated code plus a build constraint
that excludes it from compilation — alongside the manifest. This record is safe to
commit anywhere in your repository, including inside the target module, because the
build constraint prevents duplicate-symbol and orphan-package errors.

`--write` additionally installs the live test file into the target package, without
any build constraint. That is the copy that runs when you execute the tests.
Existing files and symbol conflicts are refused.

This reporter has its own schema version **4** and no measurement object.
`generated: true` with `validated: false` means generation succeeded, not that
the seeds passed: generation never executes the target and needs no API key.
Schema version 4 added support for interface parameters and the `implementations`
field in the manifest, which records the chosen concrete type for each interface
as `"Interface=Type"` strings.

Replay the generated `FuzzProfadvisor_<name>` seeds before measuring
`BenchmarkProfadvisor_<name>`. Exploration via `go test -fuzz` is a separate user
step. Import cache directories explicitly; never change the frozen corpus
between baseline and after or select cases by speed. Commit the generated tests
and manifest before `run`, which requires a clean tree. See the README for a
complete example. There is no automatic integration with `run`.

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
renormalized over what survives, and both totals are reported so you can see how
much was set aside.

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
repository, never on its working branch. Use `--dir` to name that repository;
it defaults to the current directory.

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

**The compiler is the source of truth, and an escape is not a performance
problem.** This command reports evidence and nothing else: there is no severity
field, no ranking, and no suggestion. A heap allocation on a cold path costs
nothing, and only a benchmark can say whether any of this matters — which is
what the rest of this tool is for. Do not report an allocation here as a defect.

Each finding carries a `kind` from a fixed vocabulary — `moved_to_heap`,
`escapes_to_heap`, `does_not_escape`, `leaking_param`, `leaking_param_content`,
`leaking_param_result`, `closure_capture`, and a few more — together with
`evidence`, the compiler's own line, verbatim. Match on `kind`; never on
`evidence`. Compiler diagnostic text is prose and has been reworded across
releases, which is the entire reason the `kind` field exists.

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

All three exit 0. The verdict is in the document, and a regression is a
finding, not a failure of the tool.

Do not report an optimization as done on the strength of the diagnosis alone.
