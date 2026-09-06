# Graph Report - pprof_advisor  (2026-09-06)

## Corpus Check
- 39 files · ~18,335 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 229 nodes · 492 edges · 16 communities (14 shown, 2 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 48 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `edab0304`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.T
- Run
- Commands
- Run
- memoryRepo
- newRootCmd
- FromProfile
- load
- table
- apply_test.go
- buildUserPrompt
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Resolve
- Run
- os/exec.Cmd

## God Nodes (most connected - your core abstractions)
1. `Run()` - 19 edges
2. `Run()` - 17 edges
3. `Run()` - 17 edges
4. `FromProfile()` - 16 edges
5. `realExtract()` - 11 edges
6. `load()` - 11 edges
7. `FromReaders()` - 11 edges
8. `newRootCmd()` - 10 edges
9. `ExtractResult` - 10 edges
10. `TestAppliesOnNewBranch()` - 9 edges

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

### Community 0 - "testing.T"
Cohesion: 0.16
Nodes (26): invoke(), TestCLIRejectsInvalidInputs(), TestExtractCLIEmitsV2CPU(), TestVerifyCLIObjectiveAndExitContract(), writeInput(), io.Reader, testing.T, BenchComparison (+18 more)

### Community 1 - "Run"
Cohesion: 0.19
Nodes (22): context.Context, io.Writer, time.Duration, changedFiles(), git(), gitError(), Options, nextBranch() (+14 more)

### Community 2 - "Commands"
Cohesion: 0.10
Nodes (19): Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--bench <regexp>] [--count N]`, `profadvisor extract <cpu.prof>`, `profadvisor run` (+11 more)

### Community 3 - "Run"
Cohesion: 0.14
Nodes (20): fakeClient, sdkClient, anthropic.Client, anthropic.OutputConfigEffort, Client, Options, anthropic.Message, anthropic.MessageNewParams (+12 more)

### Community 4 - "memoryRepo"
Cohesion: 0.23
Nodes (10): memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), anthropic.Message, anthropic.MessageNewParams, seedRepo(), TestPipelineAcceptsARealImprovement() (+2 more)

### Community 5 - "newRootCmd"
Cohesion: 0.19
Nodes (11): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newExtractCmd(), emit(), Execute(), newRootCmd(), newRunCmd() (+3 more)

### Community 6 - "FromProfile"
Cohesion: 0.17
Nodes (23): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Profile, benchmarkModule(), cpuSampleIndex(), FromFile(), FromProfile(), Options (+15 more)

### Community 7 - "load"
Cohesion: 0.21
Nodes (12): TestRunAgainstFixtureModule(), load(), TestAllProfRanksFixtureCode(), TestFilteringChangesTheAnswer(), TestPercentagesAreSane(), TestSourceExcerptAbsentFileIsNotAnError(), TestSourceIsAttachedFromTheFixtureTree(), TestTopNAndDeterminism() (+4 more)

### Community 8 - "table"
Cohesion: 0.27
Nodes (10): testing.B, Table, matches(), New(), segment(), BenchmarkFanout50(), BenchmarkParam(), BenchmarkStatic() (+2 more)

### Community 9 - "apply_test.go"
Cohesion: 0.52
Nodes (11): diagnosis(), gitOutput(), gitRun(), readFile(), seedRepo(), TestAppliesOnNewBranch(), TestBadDiffIsRefusedAndLeavesNoBranch(), TestBranchNumberIncrements() (+3 more)

### Community 10 - "buildUserPrompt"
Cohesion: 0.43
Nodes (5): buildUserPrompt(), nanos(), pct(), trimShape(), TestTrimShape()

### Community 13 - "Resolve"
Cohesion: 0.26
Nodes (7): measurementFlags(), Kind, Resolve(), TestDefaultsAndInvalidCombinations(), TestObjectives(), Config, Metric

### Community 14 - "Run"
Cohesion: 0.47
Nodes (4): Options, Result, Run(), TestRunCancellationKillsRunningTestProcess()

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, configureProcess(), configureProcess()

## Knowledge Gaps
- **18 isolated node(s):** `github.com/joaolaureano/profadvisor`, `example.com/profadvisor/fixture`, `What it is pointed at`, `When to use it`, `Requirements` (+13 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `testing.T`, `Run`, `memoryRepo`, `newRootCmd`, `FromProfile`?**
  _High betweenness centrality (0.110) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `Run`, `buildUserPrompt`, `newRootCmd`, `FromProfile`?**
  _High betweenness centrality (0.104) - this node is a cross-community bridge._
- **Why does `TestMatchAndParams()` connect `table` to `testing.T`?**
  _High betweenness centrality (0.094) - this node is a cross-community bridge._
- **Are the 6 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestBadDiffIsRefusedAndLeavesNoBranch()`) actually correct?**
  _`Run()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `TestAPIErrorIsWrapped()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 4 inferred relationships involving `Run()` (e.g. with `TestInvalidRunConfigurationDoesNotCapture()` and `TestMemoryPipeline()`) actually correct?**
  _`Run()` has 4 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `FromProfile()` (e.g. with `load()` and `TestNoCPUSampleType()`) actually correct?**
  _`FromProfile()` has 3 INFERRED edges - model-reasoned connections that need verification._