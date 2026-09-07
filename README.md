# profadvisor

Measures where a Go package spends time or memory, and decides whether a change
actually improved it. It has no target of its own and knows nothing about the
code it is pointed at: you give it a directory and a package pattern, and
everything it reports comes from the profile and benchmark output of that run.

**It runs entirely offline.** No API key, no network, no vendor. `net/http` is
not linked into the binary. Where a language model fits, if you want one, is
between two of its commands — `prompt` renders the question, you choose who
answers, and `apply` takes the patch that comes back.

## Scope

It reads CPU, allocation, block-contention and mutex-contention profiles over
`go test -bench`. Trace profiles are not covered, and it captures no I/O,
network or database latency — a target whose cost is there is profiled as
though it were idle.

Every command reports. One command judges: `verify` compares two benchmark
outputs and is the only place a verdict is reached.

## Install

```
go build -o profadvisor .
```

Requires a Go toolchain on PATH — the target's benchmarks have to compile.
Nothing else.

## Use

Point it at a repository with `--dir` and a package inside it with `--pkg`.

```
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/ --count 10
```

### Finding the cost

`capture` runs the benchmark once, keeping the profile and `bench.txt` from the
same run. `extract` ranks what it found:

```
./profadvisor extract profadvisor-out/<timestamp>/cpu.prof --format text
```

Runtime and standard-library frames are filtered out of the ranking, and where
that cost went is reported separately in `excluded`. That second list is often
the diagnosis: one hot function plus a wall of `runtime.concatstring2` and
`runtime.mallocgc` says the fix is about allocation, not about the loop.

### Choosing what to optimize

One flag selects the objective, and it reaches every stage: which pprof sample
type is read, which frame a cost is charged to, and which metric decides the
verdict.

```
# CPU time — the default, optimizes ns/op
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/

# allocation — optimizes B/op, or allocs/op with --unit
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/ --profile memory

# lock or channel contention
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/sync/ --profile mutex
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/sync/ --profile block
```

A memory run carries `ns/op` as a guard: it can turn the roll-up into `PIOROU`
but never into `MELHOROU`. Contention runs are judged on `ns/op` too. Recording
every contention event adds overhead, so absolute numbers from a contention
capture are not comparable to a clean run; baseline and after are captured with
identical flags.

### Asking a model, if you want one

`prompt` renders an extract document as the request a model would be given — a
system prompt stating the objective, a user prompt carrying the hotspots and
their source, and the JSON schema of the answer they ask for.

```
./profadvisor prompt extract.json --format text
```

This command sends nothing. It opens no connection and names no vendor; it
prints text. Paste it into a chat, or post it to whichever API you use. The
wording lives in a JSON catalog embedded at build time; `--prompts <file>`
renders from a different one.

### Applying and judging

A diff — from a model, from a colleague, from you — goes on its own branch:

```
./profadvisor apply patch.diff --dir /path/to/your/repo
```

It refuses a dirty tree, and if the patch fails after the branch exists it
deletes the branch and returns you to where you started. Then capture again and
compare:

```
./profadvisor verify --baseline <baseline bench.txt> --after <after bench.txt> --format text
```

That prints `MELHOROU`, `SEM DIFERENÇA` or `PIOROU` per benchmark and metric,
with the delta, a p-value and the sample counts. All three verdicts exit 0.

### Reading the output

Every command writes JSON to stdout and diagnostics to stderr. `--format text`
renders the same document for a person.

## Commands

| Command | What it does |
|---|---|
| `capture` | Runs the benchmark once; keeps the profile and `bench.txt`. |
| `extract` | Ranks hot functions, filters runtime noise, attaches source. |
| `prompt` | Renders an extract as a model request. Sends nothing. |
| `apply` | Applies a unified diff on a branch. Refuses a dirty tree. |
| `verify` | The only command that reaches a verdict, from two `bench.txt` files. |
| `escape` | Reports the compiler's escape analysis. No benchmark needed. |
| `benchgen` | Generates fuzz tests and benchmarks offline from a frozen corpus. |

[AGENTS.md](AGENTS.md) is the full reference: every flag, the I/O contract, the
schema versions, and what each verdict means.

## Escape analysis

```
./profadvisor escape --dir /path/to/your/repo
```

A narrower question that needs no benchmark and no profile: what the compiler
concluded about which values are heap-allocated. The conclusions are the
compiler's, reported as normalized JSON. There is no severity, no ranking and no
suggestion in the output. A heap allocation on a path that runs once costs
nothing measurable, and this command measures nothing — that is what the loop
above does.

## Generating benchmarks

`benchgen` turns a frozen directory of Go fuzz corpus files into a self-contained
`_test.go` file and a manifest, using native Go fuzzing and fixed templates.

```sh
./profadvisor benchgen --dir /path/to/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write
```

The generated fuzz target checks for panics and nothing else: it passes
unchanged when the function is edited to return a wrong answer.

The corpus format, the accepted parameter types, interface selection, and the
replay steps are in [AGENTS.md](AGENTS.md).

## Two workflows

Both keep the model at the end, after the measuring is done. Nothing it says
feeds back into how the numbers were produced.

### 1. Generate a change

The full pipeline. Measure, find the hot path, ask for a patch, apply it,
measure again, and let the statistics decide.

```sh
# measure the current state
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10
#   -> profadvisor-out/<t1>/{cpu.prof,bench.txt}

# rank the hot functions and attach their source
profadvisor extract profadvisor-out/<t1>/cpu.prof > extract.json

# ---- the only step a model touches ----
profadvisor prompt extract.json --format text > ask.txt
#   paste ask.txt into a chat, or POST it to your own API endpoint;
#   save the unified diff that comes back as patch.diff
# ---------------------------------------

profadvisor apply patch.diff --dir ~/svc
#   -> applied on branch profadvisor/suggestion-1

# measure again, on the branch apply left you on
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10
#   -> profadvisor-out/<t2>/{cpu.prof,bench.txt}

profadvisor verify \
  --baseline profadvisor-out/<t1>/bench.txt \
  --after    profadvisor-out/<t2>/bench.txt --format text
```

`verify` decides, not the model. If the answer is `SEM DIFERENÇA` or `PIOROU`,
delete the branch — that outcome cost one cycle and is the normal case, not a
failure. Use the same `--profile`/`--unit` on both captures; comparing a memory
run against a CPU run compares different programs.

### 2. Compare two states

No generation at all. You already have two versions — a release and its
predecessor, a branch and `main`, before and after someone else's patch — and
you want to know what moved.

```sh
git checkout main
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10   # -> <t1>

git checkout my-branch
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10   # -> <t2>

profadvisor verify \
  --baseline profadvisor-out/<t1>/bench.txt \
  --after    profadvisor-out/<t2>/bench.txt --format text
```

That is already the tabular answer: one row per benchmark and metric, with the
baseline, the after, the delta, a p-value and a per-row verdict.

The model, if you want one here, comes after that table and reads it — "which of
these regressions matters", "is this consistent with the diff" — with the
numbers already settled. To give it the *why* alongside the *what*, run
`extract` on both profiles and hand it those too:

```sh
profadvisor extract profadvisor-out/<t1>/cpu.prof > before.json
profadvisor extract profadvisor-out/<t2>/cpu.prof > after.json
profadvisor verify --baseline ... --after ... > verdict.json
#   hand verdict.json, before.json and after.json to whatever you like
```

`verify` reads benchmark output rather than profiles on purpose: a p-value needs
N samples of a metric, and a profile holds one capture. The profiles say where
the cost went; `bench.txt` says how much of it there was.

## Testing

```
go test ./...
```

The suite is self-contained. `testdata/fixture/` is a throwaway module with a
deliberately slow path matcher, and the committed profiles and benchmark output
were recorded from its benchmarks. The end-to-end tests run against that module
and against a temporary git repository the test creates, so nothing outside this
checkout is needed.

`testdata/escape/` works the same way for the escape parser. The parser tests run
against a recording and need no toolchain; one further test runs the real
compiler and compares. After a Go upgrade, that comparison is what tells you the
wording moved — re-record with `go test ./internal/escape -update` and read the
diff.

The prompt catalog is pinned the same way: `go test ./internal/prompt -update`
regenerates `testdata/prompts/`, and the diff is the review.
