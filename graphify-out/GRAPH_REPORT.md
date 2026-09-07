# Graph Report - pprof_advisor  (2026-09-07)

## Corpus Check
- 92 files · ~69,559 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 573 nodes · 1354 edges · 30 communities (26 shown, 4 thin omitted)
- Extraction: 86% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 183 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `00ec044c`
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
- buildUserPrompt
- resolveTarget
- Load

## God Nodes (most connected - your core abstractions)
1. `FromProfile()` - 25 edges
2. `Parse()` - 24 edges
3. `Text()` - 22 edges
4. `Run()` - 21 edges
5. `Run()` - 20 edges
6. `Resolve()` - 19 edges
7. `Run()` - 17 edges
8. `Load()` - 17 edges
9. `FromReaders()` - 16 edges
10. `invoke()` - 15 edges

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

## Communities (30 total, 4 thin omitted)

### Community 0 - "testing.B"
Cohesion: 0.20
Nodes (13): testing.B, BenchmarkParse(), loadFixtureContent(), rewriteFilesInFixture(), Table, matches(), New(), segment() (+5 more)

### Community 1 - "newClient"
Cohesion: 0.08
Nodes (28): fakeClient, client, init(), newClient(), TestCompleteParsesSSEStream(), TestError400ResponseIncludesBody(), TestMissingAPIKeyError(), TestRequestBodyFormat() (+20 more)

### Community 2 - "profadvisor"
Cohesion: 0.07
Nodes (28): Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor analyze <extract.json>`, `profadvisor apply <diagnosis.json>`, `profadvisor benchgen --dir <repo> --pkg <package> --func <name> --corpus <directory> --out <directory> [--write] [--impl Interface=Type ...]`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory] [--bench <regexp>] [--count N]` (+20 more)

### Community 3 - "FromReaders"
Cohesion: 0.12
Nodes (30): io.Reader, memoryRepo(), TestInvalidRunConfigurationDoesNotCapture(), TestMemoryPipeline(), gitOut(), seedRepo(), TestPipelineProducesArtifactsThatVerifyAsAnImprovement(), TestPipelineProducesArtifactsThatVerifyAsARegression() (+22 more)

### Community 4 - "Generate"
Cohesion: 0.14
Nodes (25): Manifest, Options, Seed, Target, Options, Result, inertCopy(), TestArtifactsCollisionAndInstall() (+17 more)

### Community 5 - "github.com/spf13/cobra.Command"
Cohesion: 0.13
Nodes (18): newAnalyzeCmd(), newApplyCmd(), newBenchgenCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), credentialsHelp() (+10 more)

### Community 6 - "FromProfile"
Cohesion: 0.12
Nodes (31): functionStats, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, compareGolden(), TestPromptsMatchGolden(), TestRunAgainstFixtureModule() (+23 more)

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
Cohesion: 0.15
Nodes (21): Options, Run(), corpusDir(), TestRunAgainstTheCorpus(), TestRunFailsOnATargetThatDoesNotCompile(), TestRunLeavesNothingUnrecognized(), TestRunRejectsAMissingDirectory(), TestRunSortsFindingsDeterministically() (+13 more)

### Community 14 - "testing.T"
Cohesion: 0.08
Nodes (59): TestBenchgenCLI(), TestBenchgenCLIImplFlagRepeatable(), TestBenchgenCLIImplFlagValidation(), TestBenchgenCLIInterfaceParameter(), TestBenchgenCLIRequiredFlags(), invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI() (+51 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.08
Nodes (46): position, Result, versionTuple, checkVersion(), hasAnyPrefix(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+38 more)

### Community 26 - "Resolve"
Cohesion: 0.11
Nodes (26): Options, Result, io.Writer, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Options, Result (+18 more)

### Community 27 - "buildUserPrompt"
Cohesion: 0.14
Nodes (15): promptBuilder, Config, strings.Builder, buildUserPrompt(), Coster(), Dec1(), Nanos(), Pct() (+7 more)

### Community 28 - "resolveTarget"
Cohesion: 0.17
Nodes (20): listedPackage, go/ast.File, go/types.Interface, go/types.Named, go/types.Package, go/types.Type, collectPackageLevelNames(), flattenFuzzArgument() (+12 more)

### Community 29 - "Load"
Cohesion: 0.11
Nodes (28): modelOptions, github.com/joaolaureano/profadvisor/internal/analyze.Client, BenchmarkBuildUserPrompt(), BenchmarkSystemPrompt(), systemPrompt(), extractPlaceholders(), Catalog, Load() (+20 more)

## Knowledge Gaps
- **30 isolated node(s):** `github.com/joaolaureano/profadvisor`, `listedPackage`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture` (+25 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `Run`, `testing.T`, `Parse`, `Resolve`, `Load`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `testing.B`, `Run`, `testing.T`?**
  _High betweenness centrality (0.057) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `newClient`, `github.com/spf13/cobra.Command`, `Run`, `Resolve`, `buildUserPrompt`, `Load`?**
  _High betweenness centrality (0.049) - this node is a cross-community bridge._
- **Are the 4 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 4 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Text()` (e.g. with `TestTextApply()` and `TestTextBenchgenWithImplementations()`) actually correct?**
  _`Text()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestApplyIgnoresUnrelatedUntrackedFiles()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._