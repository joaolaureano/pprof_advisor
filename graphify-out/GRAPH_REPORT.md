# Graph Report - pprof_advisor  (2026-09-07)

## Corpus Check
- 92 files · ~74,313 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 580 nodes · 1379 edges · 29 communities (25 shown, 4 thin omitted)
- Extraction: 86% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 187 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `21be9996`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.T
- newClient
- profadvisor
- FromReaders
- Resolve
- github.com/spf13/cobra.Command
- FromProfile
- Run
- resolveTarget
- Run
- example.com/profadvisor/fixture
- github.com/joaolaureano/profadvisor
- Run
- Generate
- os/exec.Cmd
- Parse
- escape/README.md
- example.com/escapecorpus
- testing.B
- Text
- corpus.go

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 26 edges
2. `Parse()` - 24 edges
3. `Resolve()` - 24 edges
4. `Text()` - 22 edges
5. `Run()` - 21 edges
6. `Run()` - 20 edges
7. `Run()` - 17 edges
8. `Load()` - 17 edges
9. `Config` - 16 edges
10. `FromReaders()` - 16 edges

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

### Community 0 - "testing.T"
Cohesion: 0.07
Nodes (56): TestBenchgenCLI(), TestBenchgenCLIImplFlagRepeatable(), TestBenchgenCLIImplFlagValidation(), TestBenchgenCLIInterfaceParameter(), TestBenchgenCLIRequiredFlags(), invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI() (+48 more)

### Community 1 - "newClient"
Cohesion: 0.09
Nodes (21): fakeClient, client, init(), newClient(), TestCompleteParsesSSEStream(), TestError400ResponseIncludesBody(), TestMissingAPIKeyError(), TestRequestBodyFormat() (+13 more)

### Community 2 - "profadvisor"
Cohesion: 0.06
Nodes (29): Caveats on contention profiles, Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor benchgen --dir <repo> --pkg <package> --func <name> --corpus <directory> --out <directory> [--write] [--impl Interface=Type ...]` (+21 more)

### Community 3 - "FromReaders"
Cohesion: 0.12
Nodes (31): io.Reader, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), seedRepo(), TestPipelineProducesArtifactsThatVerifyAsAnImprovement(), TestPipelineProducesArtifactsThatVerifyAsARegression() (+23 more)

### Community 4 - "Resolve"
Cohesion: 0.12
Nodes (25): io.Writer, Options, Options, Result, profileFile(), profileFlags(), Run(), TestProfileFlagsPerKind() (+17 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.09
Nodes (27): newAnalyzeCmd(), newApplyCmd(), newBenchgenCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), credentialsHelp() (+19 more)

### Community 6 - "FromProfile"
Cohesion: 0.21
Nodes (23): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), attributeFromFocus(), benchmarkModule() (+15 more)

### Community 7 - "Run"
Cohesion: 0.26
Nodes (23): context.Context, changedFiles(), git(), gitError(), nextBranch(), numstat(), rollback(), Run() (+15 more)

### Community 8 - "resolveTarget"
Cohesion: 0.09
Nodes (37): listedPackage, go/ast.File, go/types.Interface, go/types.Named, go/types.Package, go/types.Type, renderCallArguments(), TestRenderCallArgumentsKeepsTwoDigitPlaceholdersDistinct() (+29 more)

### Community 10 - "Run"
Cohesion: 0.31
Nodes (12): Options, responseSchema(), Run(), realExtract(), TestAPIErrorIsWrapped(), TestEmptyDiffIsRejected(), TestNoHotspotsIsAnError(), TestNonJSONResponseIsAnError() (+4 more)

### Community 13 - "Run"
Cohesion: 0.11
Nodes (26): Options, Result, Options, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Run(), corpusDir() (+18 more)

### Community 14 - "Generate"
Cohesion: 0.10
Nodes (31): Manifest, Options, Seed, Target, Options, Result, inertCopy(), TestArtifactsCollisionAndInstall() (+23 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.09
Nodes (43): position, Result, versionTuple, checkVersion(), hasAnyPrefix(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+35 more)

### Community 26 - "testing.B"
Cohesion: 0.17
Nodes (14): testing.B, BenchmarkParse(), loadFixtureContent(), rewriteFilesInFixture(), BenchmarkFromProfile(), Table, matches(), New() (+6 more)

### Community 27 - "Text"
Cohesion: 0.10
Nodes (32): promptBuilder, Config, strings.Builder, buildUserPrompt(), Coster(), Dec1(), Nanos(), Pct() (+24 more)

### Community 28 - "corpus.go"
Cohesion: 0.17
Nodes (23): go/ast.CallExpr, go/ast.Expr, go/token.Token, boolLiteral(), byteSliceLiteral(), charLiteral(), corpusLiteral(), decodeCorpusSeed() (+15 more)

## Knowledge Gaps
- **30 isolated node(s):** `github.com/joaolaureano/profadvisor`, `listedPackage`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture` (+25 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `testing.T`, `Resolve`, `Run`, `testing.B`, `Text`?**
  _High betweenness centrality (0.061) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `testing.B`, `Text`, `Run`?**
  _High betweenness centrality (0.056) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `testing.T`, `newClient`, `Resolve`, `github.com/spf13/cobra.Command`, `Run`, `Text`?**
  _High betweenness centrality (0.048) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `Resolve()` (e.g. with `TestContentionObjectives()` and `TestDefaultsAndInvalidCombinations()`) actually correct?**
  _`Resolve()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Text()` (e.g. with `TestTextApply()` and `TestTextBenchgenWithImplementations()`) actually correct?**
  _`Text()` has 12 INFERRED edges - model-reasoned connections that need verification._