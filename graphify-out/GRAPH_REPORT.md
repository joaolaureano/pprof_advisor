# Graph Report - pprof_advisor  (2026-09-07)

## Corpus Check
- 92 files · ~65,909 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 562 nodes · 1325 edges · 29 communities (25 shown, 4 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 177 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7e9a3910`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.B
- newClient
- profadvisor
- FromReaders
- Generate
- github.com/spf13/cobra.Command
- FromProfile
- Run
- corpus.go
- Run
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Run
- testing.T
- os/exec.Cmd
- Parse
- escape/README.md
- example.com/escapecorpus
- Resolve
- Text
- resolveTarget

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 25 edges
2. `Parse()` - 24 edges
3. `Run()` - 21 edges
4. `Run()` - 20 edges
5. `Text()` - 20 edges
6. `Resolve()` - 19 edges
7. `Run()` - 17 edges
8. `Load()` - 17 edges
9. `FromReaders()` - 16 edges
10. `Config` - 15 edges

## Surprising Connections (you probably didn't know these)
- `newAnalyzeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/analyze.go → internal/analyze/analyze.go
- `newApplyCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/apply.go → internal/apply/apply.go
- `newBenchgenCmd()` --calls--> `Generate()`  [EXTRACTED]
  cmd/benchgen.go → internal/benchgen/types.go
- `newCaptureCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/capture.go → internal/capture/capture.go
- `newEscapeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/escape.go → internal/escape/escape.go

## Import Cycles
- None detected.

## Communities (29 total, 4 thin omitted)

### Community 0 - "testing.B"
Cohesion: 0.17
Nodes (14): testing.B, BenchmarkParse(), loadFixtureContent(), rewriteFilesInFixture(), BenchmarkFromProfile(), Table, matches(), New() (+6 more)

### Community 1 - "newClient"
Cohesion: 0.09
Nodes (21): fakeClient, client, init(), newClient(), TestCompleteParsesSSEStream(), TestError400ResponseIncludesBody(), TestMissingAPIKeyError(), TestRequestBodyFormat() (+13 more)

### Community 2 - "profadvisor"
Cohesion: 0.07
Nodes (28): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor benchgen --dir <repo> --pkg <package> --func <name> --corpus <directory> --out <directory> [--write]`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]` (+20 more)

### Community 3 - "FromReaders"
Cohesion: 0.11
Nodes (32): io.Reader, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), seedRepo(), TestPipelineProducesArtifactsThatVerifyAsAnImprovement(), TestPipelineProducesArtifactsThatVerifyAsARegression() (+24 more)

### Community 4 - "Generate"
Cohesion: 0.11
Nodes (30): Manifest, Options, Seed, Target, Options, Result, inertCopy(), TestArtifactsCollisionAndInstall() (+22 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.09
Nodes (27): newAnalyzeCmd(), newApplyCmd(), newBenchgenCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), credentialsHelp() (+19 more)

### Community 6 - "FromProfile"
Cohesion: 0.21
Nodes (23): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), attributeFromFocus(), benchmarkModule() (+15 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (23): context.Context, changedFiles(), git(), gitError(), Options, nextBranch(), numstat(), rollback() (+15 more)

### Community 8 - "corpus.go"
Cohesion: 0.17
Nodes (23): go/ast.CallExpr, go/ast.Expr, go/token.Token, boolLiteral(), byteSliceLiteral(), charLiteral(), corpusLiteral(), decodeCorpusSeed() (+15 more)

### Community 10 - "Run"
Cohesion: 0.31
Nodes (12): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+4 more)

### Community 13 - "Run"
Cohesion: 0.11
Nodes (26): Options, Result, Options, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Run(), corpusDir() (+18 more)

### Community 14 - "testing.T"
Cohesion: 0.07
Nodes (61): TestBenchgenCLI(), TestBenchgenCLIRequiredFlags(), invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI(), TestExtractCLIEmitsV3CPU(), TestFormatIsValidatedBeforeAnyWork(), TestFormatJSONIsUnchanged() (+53 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.09
Nodes (43): position, Result, versionTuple, checkVersion(), hasAnyPrefix(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+35 more)

### Community 26 - "Resolve"
Cohesion: 0.13
Nodes (22): io.Writer, BenchmarkBuildUserPrompt(), BenchmarkSystemPrompt(), compareGolden(), TestPromptsMatchGolden(), systemPrompt(), Options, Result (+14 more)

### Community 27 - "Text"
Cohesion: 0.08
Nodes (36): promptBuilder, Config, strings.Builder, buildUserPrompt(), TestRunAgainstFixtureModule(), TestRunReportsNoMatchingBenchmarks(), Dir(), Load() (+28 more)

### Community 28 - "resolveTarget"
Cohesion: 0.22
Nodes (14): listedPackage, go/ast.File, go/types.Package, go/types.Type, collectPackageLevelNames(), flattenFuzzArgument(), fuzzInputType(), Options (+6 more)

## Knowledge Gaps
- **30 isolated node(s):** `github.com/joaolaureano/profadvisor`, `listedPackage`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture` (+25 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `testing.B`, `Run`, `testing.T`, `Resolve`, `Text`?**
  _High betweenness centrality (0.063) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `testing.B`, `Text`, `Run`?**
  _High betweenness centrality (0.058) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `newClient`, `github.com/spf13/cobra.Command`, `Run`, `testing.T`, `Resolve`, `Text`?**
  _High betweenness centrality (0.050) - this node is a cross-community bridge._
- **Are the 4 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 4 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestApplyIgnoresUnrelatedUntrackedFiles()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `buildUserPrompt()` and `systemPrompt()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._