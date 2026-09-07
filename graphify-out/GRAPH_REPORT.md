# Graph Report - pprof_advisor  (2026-09-06)

## Corpus Check
- 72 files · ~47,259 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 413 nodes · 934 edges · 27 communities (23 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 110 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2d315f80`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Run
- modelFlags
- profadvisor
- Run
- Run
- github.com/spf13/cobra.Command
- FromProfile
- Run
- table
- Catalog
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
1. `Parse()` - 22 edges
2. `FromProfile()` - 21 edges
3. `Run()` - 20 edges
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

### Community 0 - "Run"
Cohesion: 0.47
Nodes (4): Options, Result, Run(), TestRunCancellationKillsRunningTestProcess()

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
Nodes (29): io.Reader, io.Writer, Options, Result, profileFile(), Run(), TestWrongProfileKindIsRejected(), Config (+21 more)

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

### Community 10 - "Catalog"
Cohesion: 0.12
Nodes (16): render, strings.Builder, buildUserPrompt(), coster(), dec1(), compareGolden(), TestPromptsMatchGolden(), nanos() (+8 more)

### Community 13 - "Run"
Cohesion: 0.15
Nodes (22): Options, time.Duration, Run(), corpusDir(), TestRunAgainstTheCorpus(), TestRunFailsOnATargetThatDoesNotCompile(), TestRunLeavesNothingUnrecognized(), TestRunRejectsAMissingDirectory() (+14 more)

### Community 14 - "testing.T"
Cohesion: 0.10
Nodes (44): invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI(), TestExtractCLIEmitsV3CPU(), TestVerifyCLIObjectiveAndExitContract(), writeInput(), testing.T, memoryRepo() (+36 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.13
Nodes (33): explanationHeadingInfo, Result, versionTuple, checkVersion(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic(), isNewerThan() (+25 more)

### Community 26 - "load"
Cohesion: 0.21
Nodes (12): TestRunAgainstFixtureModule(), load(), TestAllProfRanksFixtureCode(), TestFilteringChangesTheAnswer(), TestPercentagesAreSane(), TestSourceExcerptAbsentFileIsNotAnError(), TestSourceIsAttachedFromTheFixtureTree(), TestTopNAndDeterminism() (+4 more)

## Knowledge Gaps
- **25 isolated node(s):** `github.com/joaolaureano/profadvisor`, `example.com/escapecorpus`, `example.com/profadvisor/fixture`, `What it is pointed at`, `When to use it` (+20 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `modelFlags`, `Run`, `github.com/spf13/cobra.Command`, `Run`, `Catalog`, `testing.T`?**
  _High betweenness centrality (0.070) - this node is a cross-community bridge._
- **Why does `FromProfile()` connect `FromProfile` to `load`, `Catalog`, `Run`, `Run`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Run`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `Parse()` (e.g. with `Run()` and `TestParseAttachesFlowAndFunction()`) actually correct?**
  _`Parse()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `FromProfile()` (e.g. with `load()` and `TestSourceExcerptAbsentFileIsNotAnError()`) actually correct?**
  _`FromProfile()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestBadDiffIsRefusedAndLeavesNoBranch()`) actually correct?**
  _`Run()` has 6 INFERRED edges - model-reasoned connections that need verification._