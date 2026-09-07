# Graph Report - pprof_advisor  (2026-09-07)

## Corpus Check
- 78 files · ~54,993 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 454 nodes · 1066 edges · 27 communities (23 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 126 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `73c07519`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Run
- modelFlags
- profadvisor
- FromReaders
- Resolve
- github.com/spf13/cobra.Command
- FromProfile
- Run
- Run
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Run
- testing.T
- os/exec.Cmd
- Parse
- escape/README.md
- example.com/escapecorpus
- load
- buildUserPrompt

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 25 edges
2. `Parse()` - 24 edges
3. `Run()` - 21 edges
4. `Run()` - 20 edges
5. `Text()` - 19 edges
6. `Resolve()` - 18 edges
7. `Run()` - 17 edges
8. `Load()` - 17 edges
9. `Config` - 15 edges
10. `buildUserPrompt()` - 14 edges

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
Nodes (32): fakeClient, client, credentialsHelp(), joinOr(), modelFlags(), modelOptions, init(), newClient() (+24 more)

### Community 2 - "profadvisor"
Cohesion: 0.07
Nodes (26): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]`, `profadvisor escape [--dir <repo>] [--pkg <pattern>] [--timeout <d>]` (+18 more)

### Community 3 - "FromReaders"
Cohesion: 0.13
Nodes (28): io.Reader, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), seedRepo(), TestPipelineProducesArtifactsThatVerifyAsAnImprovement(), TestPipelineProducesArtifactsThatVerifyAsARegression() (+20 more)

### Community 4 - "Resolve"
Cohesion: 0.18
Nodes (17): io.Writer, Options, Result, profileFile(), Run(), Config, Kind, Metric (+9 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.16
Nodes (14): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), emit(), Execute() (+6 more)

### Community 6 - "FromProfile"
Cohesion: 0.21
Nodes (23): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), attributeFromFocus(), benchmarkModule() (+15 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (23): context.Context, changedFiles(), git(), gitError(), Options, nextBranch(), numstat(), rollback() (+15 more)

### Community 10 - "Run"
Cohesion: 0.13
Nodes (26): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+18 more)

### Community 13 - "Run"
Cohesion: 0.15
Nodes (22): Options, time.Duration, Run(), corpusDir(), TestRunAgainstTheCorpus(), TestRunFailsOnATargetThatDoesNotCompile(), TestRunLeavesNothingUnrecognized(), TestRunRejectsAMissingDirectory() (+14 more)

### Community 14 - "testing.T"
Cohesion: 0.10
Nodes (46): invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI(), TestExtractCLIEmitsV3CPU(), TestFormatIsValidatedBeforeAnyWork(), TestFormatJSONIsUnchanged(), TestFormatTextIsHumanReadable(), TestVerifyCLIObjectiveAndExitContract() (+38 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.13
Nodes (31): explanationHeadingInfo, position, Result, versionTuple, checkVersion(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+23 more)

### Community 26 - "load"
Cohesion: 0.08
Nodes (32): testing.B, BenchmarkBuildUserPrompt(), BenchmarkSystemPrompt(), compareGolden(), TestPromptsMatchGolden(), systemPrompt(), TestRunAgainstFixtureModule(), BenchmarkParse() (+24 more)

### Community 27 - "buildUserPrompt"
Cohesion: 0.13
Nodes (17): promptBuilder, Config, strings.Builder, buildUserPrompt(), Coster(), Dec1(), Nanos(), Pct() (+9 more)

## Knowledge Gaps
- **27 isolated node(s):** `github.com/joaolaureano/profadvisor`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture`, `What it is pointed at` (+22 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `Run`, `load`, `Resolve`, `testing.T`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `load`, `Run`, `testing.T`?**
  _High betweenness centrality (0.068) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `modelFlags`, `Resolve`, `github.com/spf13/cobra.Command`, `Run`, `testing.T`, `load`, `buildUserPrompt`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Are the 4 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 4 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestApplyIgnoresUnrelatedUntrackedFiles()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._