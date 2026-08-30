# profadvisor

A CLI that finds a CPU hot-path in a Go benchmark, asks Claude how to fix it,
and then measures whether the fix actually worked.

Agents should reach for this tool when someone asks *why* Go code is slow, or
asks to make a CPU-bound Go package faster. It is not a profiler viewer and not
a general optimizer: it only answers questions a benchmark can settle.

## What it is pointed at

profadvisor is target-agnostic and stateless about targets. It has no
configured project, no default repository, and no knowledge of any codebase
other than the one named on the command line:

- `--dir` is the root of the repository under test. It defaults to the current
  directory, so pass it explicitly whenever the target is elsewhere.
- `--pkg` is a `go test` package pattern resolved **inside `--dir`**, not
  inside this tool's checkout.

Everything reported — hotspots, module name, source excerpts, the verdict —
comes from that run. If you are reading this file as a reference while working
on some other repository, the target is that repository: pass its path as
`--dir`. Nothing in this document names a project to run against, and the
examples below use placeholder paths that you are expected to replace.

## When to use it

Use it when **all** of these hold:

- The target is a Go package with benchmarks (`go test -bench`), or you can write them.
- The work is CPU-bound — algorithms, parsing, routing, serialization, business logic.
- The question is "what is slow and does this change help", not "is this design good".

Do **not** use it when:

- The suspected cost is I/O, network, database, lock contention, or memory
  pressure. V1 captures a CPU profile only; those need a block, mutex, or heap
  profile and will produce a confidently wrong answer here.
- There is no benchmark and the code cannot be exercised deterministically.
  Without a benchmark there is nothing to compare against, and the verdict step —
  the only part that establishes a change was worth making — cannot run.

## Requirements

- A Go toolchain on PATH. The target's benchmark has to compile, so this is
  structural; nothing else needs installing.
- The target repository is a git repository if you intend to use `apply` or
  `run`; the diff is applied on a branch there, and a dirty tree is refused.
- `ANTHROPIC_API_KEY`, or credentials from `ant auth login`. Needed by `analyze`
  only; `capture`, `extract`, and `verify` work offline.

## I/O contract

Every subcommand follows the same rules, and tooling can rely on them:

- **stdout** carries the result as indented JSON, and carries nothing else.
- **stderr** carries every diagnostic, progress line, and error message.
- **exit 0** means the JSON on stdout is complete. **Non-zero** means it is
  absent or partial — do not parse stdout after a non-zero exit.

Each document carries `schema_version`. A consumer that does not recognize the
version should stop rather than guess at the shape.

## Commands

### `profadvisor capture --pkg <pattern> [--dir <repo>] [--bench <regexp>] [--count N]`

Runs the benchmark once and writes a timestamped directory under
`./profadvisor-out/` containing `cpu.prof` and `bench.txt`.

Both artifacts come from the same run, deliberately: `cpu.prof` feeds the
diagnosis and `bench.txt` feeds the verdict, and if they came from different runs
they would describe different programs.

`--count` defaults to 10. Lowering it makes `verify` less able to distinguish a
real change from noise; a single sample has more than once suggested a
regression that turned out to be a double-digit gain.

```
profadvisor capture --dir /path/to/target --pkg ./internal/parser/ --bench '^BenchmarkParse$'
```

### `profadvisor extract <cpu.prof>`

Ranks the hottest functions by self time and attaches the surrounding source
with per-line cost.

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

The output also reports **where the filtered time went**, hottest by cumulative
time. This matters because filtering can remove the explanation along with the
noise: a loop that boxes a value into an interface shows up as one hot function
plus a wall of `runtime.convTnoptr` and `runtime.mallocgc`, and only the second
half says that the fix is about allocation.

### `profadvisor analyze <extract.json>`

Sends the hotspots and their source to Claude and returns a diagnosis plus a
unified diff.

**The output is a hypothesis.** Nothing in it has been measured, and the model's
own `confidence` field is not evidence. A suggestion only counts once `verify`
confirms it. Treat a high-confidence diff that fails verification as a normal
outcome, not a bug.

### `profadvisor apply <diagnosis.json>`

Applies the diff on a new branch, `profadvisor/suggestion-N`, in the target
repository, never on its working branch. Use `--dir` to name that repository;
it defaults to the current directory.

### `profadvisor verify --baseline <bench.txt> --after <bench.txt>`

Compares two benchmark outputs and returns `MELHOROU`, `SEM DIFERENÇA`, or
`PIOROU` per benchmark, with the percentage delta and a p-value.

Note the inputs: this compares **benchmark output**, not profiles. A p-value
needs N samples of `ns/op`, which a CPU profile does not contain — a profile
says where time went, not how long the operation took. Capture both with
`--count 10` or higher on each side.

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
  a malfunction.

Do not report an optimization as done on the strength of the diagnosis alone.
