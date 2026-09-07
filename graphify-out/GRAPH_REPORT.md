# Graph Report - pprof_advisor-goroutine  (2026-09-07)

## Corpus Check
- 78 files · ~60,124 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 465 nodes · 1105 edges · 27 communities (23 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 134 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `14fb4748`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Load
- newClient
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
- testing.B
- Text

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 26 edges
2. `Parse()` - 24 edges
3. `Resolve()` - 24 edges
4. `Run()` - 21 edges
5. `Run()` - 20 edges
6. `Text()` - 19 edges
7. `Run()` - 17 edges
8. `Load()` - 17 edges
9. `Config` - 16 edges
10. `FromReaders()` - 15 edges

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

### Community 0 - "Load"
Cohesion: 0.11
Nodes (28): modelOptions, github.com/joaolaureano/profadvisor/internal/analyze.Client, BenchmarkBuildUserPrompt(), BenchmarkSystemPrompt(), systemPrompt(), extractPlaceholders(), Catalog, Load() (+20 more)

### Community 1 - "newClient"
Cohesion: 0.08
Nodes (25): fakeClient, client, init(), newClient(), TestCompleteParsesSSEStream(), TestError400ResponseIncludesBody(), TestMissingAPIKeyError(), TestRequestBodyFormat() (+17 more)

### Community 2 - "profadvisor"
Cohesion: 0.07
Nodes (27): Caveats on contention profiles, Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory|block|mutex] [--bench <regexp>] [--count N] [--rate N]` (+19 more)

### Community 3 - "FromReaders"
Cohesion: 0.12
Nodes (31): io.Reader, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), seedRepo(), TestPipelineProducesArtifactsThatVerifyAsAnImprovement(), TestPipelineProducesArtifactsThatVerifyAsARegression() (+23 more)

### Community 4 - "Resolve"
Cohesion: 0.15
Nodes (22): io.Writer, Options, Result, profileFile(), profileFlags(), Run(), TestProfileFlagsPerKind(), Config (+14 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.14
Nodes (18): newAnalyzeCmd(), newApplyCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), credentialsHelp(), joinOr() (+10 more)

### Community 6 - "FromProfile"
Cohesion: 0.22
Nodes (22): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), attributeFromFocus(), benchmarkModule() (+14 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (23): context.Context, changedFiles(), git(), gitError(), Options, nextBranch(), numstat(), rollback() (+15 more)

### Community 10 - "Run"
Cohesion: 0.13
Nodes (26): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+18 more)

### Community 13 - "Run"
Cohesion: 0.11
Nodes (26): Options, Result, Options, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Run(), corpusDir() (+18 more)

### Community 14 - "testing.T"
Cohesion: 0.12
Nodes (31): invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI(), TestExtractCLIEmitsV3CPU(), TestFormatIsValidatedBeforeAnyWork(), TestFormatJSONIsUnchanged(), TestFormatTextIsHumanReadable(), TestVerifyCLIObjectiveAndExitContract() (+23 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.14
Nodes (30): position, Result, versionTuple, checkVersion(), hasAnyPrefix(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+22 more)

### Community 26 - "testing.B"
Cohesion: 0.17
Nodes (14): testing.B, BenchmarkParse(), loadFixtureContent(), rewriteFilesInFixture(), BenchmarkFromProfile(), Table, matches(), New() (+6 more)

### Community 27 - "Text"
Cohesion: 0.10
Nodes (29): promptBuilder, Config, strings.Builder, buildUserPrompt(), Coster(), Dec1(), Nanos(), Pct() (+21 more)

## Knowledge Gaps
- **27 isolated node(s):** `github.com/joaolaureano/profadvisor`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture`, `What it is pointed at` (+22 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `Load`, `Resolve`, `Run`, `testing.T`, `testing.B`, `Text`?**
  _High betweenness centrality (0.072) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Text`, `testing.B`, `Run`, `Run`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `Load`, `newClient`, `Resolve`, `github.com/spf13/cobra.Command`, `Run`, `Text`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `Resolve()` (e.g. with `TestContentionObjectives()` and `TestDefaultsAndInvalidCombinations()`) actually correct?**
  _`Resolve()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestApplyIgnoresUnrelatedUntrackedFiles()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._