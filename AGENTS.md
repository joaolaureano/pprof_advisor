# profadvisor

A CLI that finds a hot-path in a Go benchmark, asks a language model how to fix
it, and
then measures whether the fix actually worked. It optimizes either CPU time or
memory allocation, chosen per run.

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

Do **not** use it when:

- The suspected cost is I/O, network, database, or lock contention. Those need a
  block or mutex profile, which this tool does not capture, and it will produce
  a confidently wrong answer.
- There is no benchmark and the code cannot be exercised deterministically.
  Without a benchmark there is nothing to compare against, and the verdict step —
  the only part that establishes a change was worth making — cannot run.

## Choosing the objective

`--profile cpu` (the default) optimizes `ns/op` from a CPU profile.
`--profile memory` optimizes `B/op` from an allocation profile, or `allocs/op`
with `--unit allocs/op`. The profile can be left off when the unit implies it:
`--unit B/op` alone means a memory run.

The objective is set once and reaches every stage: it selects which pprof sample
type is read, which frame a cost is charged to, how the prompt frames the task,
and which metric decides the verdict. That is why it is one flag and not four.

**A memory run keeps `ns/op` as a guard.** Trading time for memory is easy and
usually not what was asked for, so a patch that significantly slows the
benchmark is `PIOROU` even when it allocates less. The two memory units do not
guard each other: fewer bytes in more allocations is a legitimate outcome, so
the unit you did not choose is reported and votes on nothing.

Allocation profiles are attributed differently from CPU profiles, and this shows
up in the output. The leaf frame of every allocation sample is
`runtime.mallocgc`, so charging cost to the leaf — correct for CPU, where the
leaf is the code that was running — would rank the allocator first and the code
under test nowhere. Instead each sample is charged to the innermost frame inside
the focus package: the call that asked for the memory, which is the line a patch
can change.

## Requirements

- A Go toolchain on PATH. The target's benchmark has to compile, so this is
  structural; nothing else needs installing.
- The target repository is a git repository if you intend to use `apply` or
  `run`; the diff is applied on a branch there, and a dirty tree is refused.
- An API key for the selected model provider: `PROFADVISOR_API_KEY`, or that
  provider's own conventional variable. Needed by `analyze` and `run` only;
  `capture`, `extract`, and `verify` work offline.

## I/O contract

Every subcommand follows the same rules, and tooling can rely on them:

- **stdout** carries the result as indented JSON, and carries nothing else.
- **stderr** carries every diagnostic, progress line, and error message.
- **exit 0** means the JSON on stdout is complete. **Non-zero** means it is
  absent or partial — do not parse stdout after a non-zero exit.

Each document carries `schema_version`, currently **2**. A consumer that does
not recognize the version should stop rather than guess at the shape.

Every document also carries a `measurement` object — profile, unit, pprof sample
type, sample unit, and attribution rule. Cost fields (`total`, `analyzed`,
`flat`, `cum`, `line_costs`) are plain numbers in `measurement.sample_unit`:
nanoseconds for a CPU run, bytes or object counts for a memory one. **Do not
assume nanoseconds.** Version 1 named these fields `*_nanos` and had no
`measurement`; that is the whole difference, and why the version moved.

## Commands

### `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]`

Runs the benchmark once and writes a timestamped directory under
`./profadvisor-out/` containing the profile — `cpu.prof` or `mem.prof`, per
`--profile` — and `bench.txt`.

Both artifacts come from the same run, deliberately: the profile feeds the
diagnosis and `bench.txt` feeds the verdict, and if they came from different runs
they would describe different programs.

`-benchmem` is always on, whatever the objective, because `verify` needs
`ns/op`, `B/op` and `allocs/op` from one output in order to guard one against
another.

`--count` defaults to 10. Lowering it makes `verify` less able to distinguish a
real change from noise; a single sample has more than once suggested a
regression that turned out to be a double-digit gain.

```
profadvisor capture --dir /path/to/target --pkg ./internal/parser/ --bench '^BenchmarkParse$'
```

### `profadvisor extract <profile> [--profile cpu|memory]`

Ranks the hottest functions by self cost and attaches the surrounding source
with per-line cost. `--profile` must match the profile that was captured;
pointing a CPU objective at `mem.prof` is an error rather than an empty result.

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

The output also reports **where the filtered cost went**, largest by cumulative
cost. This matters because filtering can remove the explanation along with the
noise: a loop that boxes a value into an interface shows up as one hot function
plus a wall of `runtime.convTnoptr` and `runtime.mallocgc`, and only the second
half says that the fix is about allocation.

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

### `profadvisor run`

Runs the five steps in order. Use the individual commands when you want to
inspect or edit anything in between — in particular, reading the diff before
applying it is usually worth the extra step.

## What a result means

The pipeline succeeds only when `verify` reports `MELHOROU`. Every other outcome
is information, not failure:

- `SEM DIFERENÇA` — the change was neutral. Discard the branch; the hypothesis
  was wrong, and knowing that cost one cycle.
- `PIOROU` — the change was harmful. Discard the branch. This happens and is not
  a malfunction. On a memory run, check which metric caused it: a `PIOROU` from
  the `ns/op` guard with the objective improving means the model bought memory
  with time, and is worth re-running with a narrower `--bench`.

Do not report an optimization as done on the strength of the diagnosis alone.
