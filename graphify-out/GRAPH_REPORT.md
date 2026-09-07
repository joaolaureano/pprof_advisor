# Graph Report - pprof_advisor  (2026-09-06)

## Corpus Check
- 54 files · ~32,216 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 316 nodes · 734 edges · 16 communities (14 shown, 2 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 85 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `83266499`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- FromReaders
- newClient
- profadvisor
- Run
- Run
- github.com/spf13/cobra.Command
- FromProfile
- Run
- table
- buildUserPrompt
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Config
- testing.T
- os/exec.Cmd

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 21 edges
2. `Run()` - 20 edges
3. `Run()` - 19 edges
4. `Run()` - 18 edges
5. `Config` - 16 edges
6. `Resolve()` - 15 edges
7. `Load()` - 15 edges
8. `Catalog` - 14 edges
9. `buildUserPrompt()` - 13 edges
10. `LoadFile()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `newAnalyzeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/analyze.go → internal/analyze/analyze.go
- `newApplyCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/apply.go → internal/apply/apply.go
- `newCaptureCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/capture.go → internal/capture/capture.go
- `newExtractCmd()` --calls--> `FromFile()`  [EXTRACTED]
  cmd/extract.go → internal/extract/extract.go
- `newExtractCmd()` --calls--> `Resolve()`  [EXTRACTED]
  cmd/extract.go → internal/measurement/measurement.go

## Import Cycles
- None detected.

## Communities (16 total, 2 thin omitted)

### Community 0 - "FromReaders"
Cohesion: 0.18
Nodes (21): io.Reader, BenchComparison, VerifyResult, compare(), finitePValue(), FromFiles(), FromReaders(), Options (+13 more)

### Community 1 - "newClient"
Cohesion: 0.08
Nodes (28): fakeClient, client, init(), newClient(), TestCompleteParsesSSEStream(), TestError400ResponseIncludesBody(), TestMissingAPIKeyError(), TestRequestBodyFormat() (+20 more)

### Community 2 - "profadvisor"
Cohesion: 0.08
Nodes (22): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]`, `profadvisor extract <profile> [--profile cpu|memory]` (+14 more)

### Community 3 - "Run"
Cohesion: 0.33
Nodes (11): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+3 more)

### Community 4 - "Run"
Cohesion: 0.24
Nodes (13): io.Writer, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), checkout(), firstNonEmpty(), resolve(), Run() (+5 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.15
Nodes (16): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newExtractCmd(), measurementFlags(), credentialsHelp(), joinOr(), modelFlags() (+8 more)

### Community 6 - "FromProfile"
Cohesion: 0.21
Nodes (22): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), benchmarkModule(), FromFile() (+14 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (21): context.Context, changedFiles(), git(), gitError(), Options, nextBranch(), numstat(), rollback() (+13 more)

### Community 8 - "table"
Cohesion: 0.27
Nodes (10): testing.B, Table, matches(), New(), segment(), BenchmarkFanout50(), BenchmarkParam(), BenchmarkStatic() (+2 more)

### Community 10 - "buildUserPrompt"
Cohesion: 0.12
Nodes (17): render, strings.Builder, buildUserPrompt(), coster(), dec1(), compareGolden(), TestPromptsMatchGolden(), nanos() (+9 more)

### Community 13 - "Config"
Cohesion: 0.12
Nodes (23): Options, Result, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Options, Result, profileFile() (+15 more)

### Community 14 - "testing.T"
Cohesion: 0.10
Nodes (39): invoke(), TestCLIRejectsInvalidInputs(), TestExtractCLIEmitsV3CPU(), TestVerifyCLIObjectiveAndExitContract(), writeInput(), modelOptions, github.com/joaolaureano/profadvisor/internal/analyze.Client, testing.T (+31 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, configureProcess(), configureProcess()

## Knowledge Gaps
- **21 isolated node(s):** `github.com/joaolaureano/profadvisor`, `example.com/profadvisor/fixture`, `What it is pointed at`, `When to use it`, `Choosing the objective` (+16 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `newClient`, `Run`, `github.com/spf13/cobra.Command`, `Run`, `buildUserPrompt`, `Config`, `testing.T`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Why does `FromProfile()` connect `FromProfile` to `buildUserPrompt`, `Run`, `Config`, `testing.T`?**
  _High betweenness centrality (0.090) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `FromReaders`, `Run`, `github.com/spf13/cobra.Command`, `FromProfile`, `Run`, `Config`, `testing.T`?**
  _High betweenness centrality (0.072) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `FromProfile()` (e.g. with `load()` and `TestSourceExcerptAbsentFileIsNotAnError()`) actually correct?**
  _`FromProfile()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestBadDiffIsRefusedAndLeavesNoBranch()`) actually correct?**
  _`Run()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **Are the 4 inferred relationships involving `Run()` (e.g. with `TestInvalidRunConfigurationDoesNotCapture()` and `TestMemoryPipeline()`) actually correct?**
  _`Run()` has 4 INFERRED edges - model-reasoned connections that need verification._