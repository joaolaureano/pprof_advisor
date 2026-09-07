# Graph Report - pprof_advisor  (2026-09-06)

## Corpus Check
- 72 files · ~46,370 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 411 nodes · 928 edges · 27 communities (23 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 108 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4ce38c79`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- FromReaders
- modelFlags
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
- Run
- testing.T
- os/exec.Cmd
- Parse
- escape/README.md
- example.com/escapecorpus
- load

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 21 edges
2. `Run()` - 20 edges
3. `Parse()` - 20 edges
4. `Run()` - 19 edges
5. `Run()` - 18 edges
6. `Config` - 16 edges
7. `Resolve()` - 15 edges
8. `Load()` - 15 edges
9. `Catalog` - 14 edges
10. `buildUserPrompt()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `newAnalyzeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/analyze.go → internal/analyze/analyze.go
- `newApplyCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/apply.go → internal/apply/apply.go
- `newCaptureCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/capture.go → internal/capture/capture.go
- `newEscapeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/escape.go → internal/escape/escape.go
- `newExtractCmd()` --calls--> `FromFile()`  [EXTRACTED]
  cmd/extract.go → internal/extract/extract.go

## Import Cycles
- None detected.

## Communities (27 total, 4 thin omitted)

### Community 0 - "FromReaders"
Cohesion: 0.18
Nodes (21): io.Reader, BenchComparison, VerifyResult, compare(), finitePValue(), FromFiles(), FromReaders(), Options (+13 more)

### Community 1 - "modelFlags"
Cohesion: 0.07
Nodes (33): fakeClient, client, credentialsHelp(), joinOr(), modelFlags(), modelOptions, github.com/joaolaureano/profadvisor/internal/analyze.Client, init() (+25 more)

### Community 2 - "profadvisor"
Cohesion: 0.08
Nodes (24): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]`, `profadvisor escape [--dir <repo>] [--pkg <pattern>] [--timeout <d>]` (+16 more)

### Community 3 - "Run"
Cohesion: 0.15
Nodes (23): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+15 more)

### Community 4 - "Run"
Cohesion: 0.12
Nodes (24): Options, Result, io.Writer, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Options, Result (+16 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.16
Nodes (14): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), emit(), Execute() (+6 more)

### Community 6 - "FromProfile"
Cohesion: 0.22
Nodes (21): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), benchmarkModule(), FromFile() (+13 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (21): context.Context, changedFiles(), git(), gitError(), Options, nextBranch(), numstat(), rollback() (+13 more)

### Community 8 - "table"
Cohesion: 0.27
Nodes (10): testing.B, Table, matches(), New(), segment(), BenchmarkFanout50(), BenchmarkParam(), BenchmarkStatic() (+2 more)

### Community 10 - "buildUserPrompt"
Cohesion: 0.16
Nodes (12): render, strings.Builder, buildUserPrompt(), coster(), dec1(), compareGolden(), TestPromptsMatchGolden(), nanos() (+4 more)

### Community 13 - "Run"
Cohesion: 0.15
Nodes (21): Options, Run(), corpusDir(), TestRunAgainstTheCorpus(), TestRunFailsOnATargetThatDoesNotCompile(), TestRunLeavesNothingUnrecognized(), TestRunRejectsAMissingDirectory(), TestRunSortsFindingsDeterministically() (+13 more)

### Community 14 - "testing.T"
Cohesion: 0.11
Nodes (37): invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI(), TestExtractCLIEmitsV3CPU(), TestVerifyCLIObjectiveAndExitContract(), writeInput(), testing.T, memoryRepo() (+29 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.13
Nodes (31): explanationHeadingInfo, Result, versionTuple, checkVersion(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic(), isNewerThan() (+23 more)

### Community 26 - "load"
Cohesion: 0.21
Nodes (12): TestRunAgainstFixtureModule(), load(), TestAllProfRanksFixtureCode(), TestFilteringChangesTheAnswer(), TestPercentagesAreSane(), TestSourceExcerptAbsentFileIsNotAnError(), TestSourceIsAttachedFromTheFixtureTree(), TestTopNAndDeterminism() (+4 more)

## Knowledge Gaps
- **25 isolated node(s):** `github.com/joaolaureano/profadvisor`, `example.com/escapecorpus`, `example.com/profadvisor/fixture`, `What it is pointed at`, `When to use it` (+20 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `modelFlags`, `Run`, `github.com/spf13/cobra.Command`, `Run`, `buildUserPrompt`, `testing.T`?**
  _High betweenness centrality (0.070) - this node is a cross-community bridge._
- **Why does `FromProfile()` connect `FromProfile` to `load`, `buildUserPrompt`, `Run`, `Run`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Run`?**
  _High betweenness centrality (0.059) - this node is a cross-community bridge._
- **Are the 3 inferred relationships involving `FromProfile()` (e.g. with `load()` and `TestSourceExcerptAbsentFileIsNotAnError()`) actually correct?**
  _`FromProfile()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **Are the 10 inferred relationships involving `Parse()` (e.g. with `Run()` and `TestParseAttachesFlowAndFunction()`) actually correct?**
  _`Parse()` has 10 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestBadDiffIsRefusedAndLeavesNoBranch()`) actually correct?**
  _`Run()` has 6 INFERRED edges - model-reasoned connections that need verification._