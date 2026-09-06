# Graph Report - pprof_advisor  (2026-09-06)

## Corpus Check
- 39 files · ~21,804 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 243 nodes · 559 edges · 16 communities (14 shown, 2 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 54 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f957cae7`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- FromReaders
- Run
- profadvisor
- Run
- Run
- newRootCmd
- FromProfile
- testing.T
- table
- ExtractResult
- prompt.go
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Config
- invoke
- os/exec.Cmd

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 20 edges
2. `Run()` - 19 edges
3. `Run()` - 19 edges
4. `Run()` - 18 edges
5. `Config` - 17 edges
6. `Resolve()` - 14 edges
7. `FromReaders()` - 12 edges
8. `realExtract()` - 11 edges
9. `load()` - 11 edges
10. `newRootCmd()` - 10 edges

## Surprising Connections (you probably didn't know these)
- `newAnalyzeCmd()` --calls--> `NewClient()`  [EXTRACTED]
  cmd/analyze.go → internal/analyze/analyze.go
- `newAnalyzeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/analyze.go → internal/analyze/analyze.go
- `newApplyCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/apply.go → internal/apply/apply.go
- `newCaptureCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/capture.go → internal/capture/capture.go
- `newExtractCmd()` --calls--> `FromFile()`  [EXTRACTED]
  cmd/extract.go → internal/extract/extract.go

## Import Cycles
- None detected.

## Communities (16 total, 2 thin omitted)

### Community 0 - "FromReaders"
Cohesion: 0.18
Nodes (21): io.Reader, BenchComparison, VerifyResult, compare(), finitePValue(), FromFiles(), FromReaders(), Options (+13 more)

### Community 1 - "Run"
Cohesion: 0.22
Nodes (15): fakeClient, context.Context, anthropic.Message, anthropic.MessageNewParams, changedFiles(), git(), gitError(), Options (+7 more)

### Community 2 - "profadvisor"
Cohesion: 0.09
Nodes (20): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]`, `profadvisor extract <profile> [--profile cpu|memory]` (+12 more)

### Community 3 - "Run"
Cohesion: 0.18
Nodes (17): sdkClient, anthropic.Client, anthropic.OutputConfigEffort, Client, Options, anthropic.Message, anthropic.MessageNewParams, NewClient() (+9 more)

### Community 4 - "Run"
Cohesion: 0.24
Nodes (13): io.Writer, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), checkout(), firstNonEmpty(), resolve(), Run() (+5 more)

### Community 5 - "newRootCmd"
Cohesion: 0.18
Nodes (13): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newExtractCmd(), measurementFlags(), emit(), Execute(), execute() (+5 more)

### Community 6 - "FromProfile"
Cohesion: 0.22
Nodes (21): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), benchmarkModule(), FromFile() (+13 more)

### Community 7 - "testing.T"
Cohesion: 0.18
Nodes (26): testing.T, diagnosis(), gitOutput(), gitRun(), readFile(), seedRepo(), TestAppliesOnNewBranch(), TestBadDiffIsRefusedAndLeavesNoBranch() (+18 more)

### Community 8 - "table"
Cohesion: 0.27
Nodes (10): testing.B, Table, matches(), New(), segment(), BenchmarkFanout50(), BenchmarkParam(), BenchmarkStatic() (+2 more)

### Community 9 - "ExtractResult"
Cohesion: 0.33
Nodes (9): sourceExcerpt(), ApplyResult, Diagnosis, ExcludedCost, ExtractResult, SourceExcerpt, Result, Hotspot (+1 more)

### Community 10 - "prompt.go"
Cohesion: 0.27
Nodes (8): buildUserPrompt(), coster(), guardRule(), nanos(), pct(), systemPrompt(), trimShape(), TestTrimShape()

### Community 13 - "Config"
Cohesion: 0.16
Nodes (15): Options, Result, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Options, Result, profileFile() (+7 more)

### Community 14 - "invoke"
Cohesion: 0.67
Nodes (5): invoke(), TestCLIRejectsInvalidInputs(), TestExtractCLIEmitsV2CPU(), TestVerifyCLIObjectiveAndExitContract(), writeInput()

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, configureProcess(), configureProcess()

## Knowledge Gaps
- **19 isolated node(s):** `github.com/joaolaureano/profadvisor`, `example.com/profadvisor/fixture`, `What it is pointed at`, `When to use it`, `Choosing the objective` (+14 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `Run`, `Run`, `newRootCmd`, `ExtractResult`, `prompt.go`, `Config`?**
  _High betweenness centrality (0.112) - this node is a cross-community bridge._
- **Why does `FromProfile()` connect `FromProfile` to `ExtractResult`, `Run`, `Config`, `testing.T`?**
  _High betweenness centrality (0.105) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `FromReaders`, `Run`, `Run`, `newRootCmd`, `FromProfile`, `ExtractResult`, `Config`?**
  _High betweenness centrality (0.093) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `FromProfile()` (e.g. with `load()` and `TestSourceExcerptAbsentFileIsNotAnError()`) actually correct?**
  _`FromProfile()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestBadDiffIsRefusedAndLeavesNoBranch()`) actually correct?**
  _`Run()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **Are the 4 inferred relationships involving `Run()` (e.g. with `TestInvalidRunConfigurationDoesNotCapture()` and `TestMemoryPipeline()`) actually correct?**
  _`Run()` has 4 INFERRED edges - model-reasoned connections that need verification._