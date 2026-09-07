# Graph Report - pprof_advisor  (2026-09-07)

## Corpus Check
- 74 files · ~64,616 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 499 nodes · 1140 edges · 27 communities (23 shown, 4 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 153 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2f2e642e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.T
- placeholder_test.go
- profadvisor
- FromReaders
- Resolve
- newRootCmd
- FromProfile
- Run
- resolveTarget
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
1. `Parse()` - 24 edges
2. `FromProfile()` - 24 edges
3. `Resolve()` - 20 edges
4. `Run()` - 19 edges
5. `Text()` - 19 edges
6. `FromReaders()` - 16 edges
7. `invoke()` - 15 edges
8. `resolveTarget()` - 14 edges
9. `Generate()` - 14 edges
10. `Config` - 14 edges

## Surprising Connections (you probably didn't know these)
- `newApplyCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/apply.go → internal/apply/apply.go
- `newBenchgenCmd()` --calls--> `Generate()`  [EXTRACTED]
  cmd/benchgen.go → internal/benchgen/types.go
- `newCaptureCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/capture.go → internal/capture/capture.go
- `newEscapeCmd()` --calls--> `Run()`  [EXTRACTED]
  cmd/escape.go → internal/escape/escape.go
- `newExtractCmd()` --calls--> `FromFile()`  [EXTRACTED]
  cmd/extract.go → internal/extract/extract.go

## Import Cycles
- None detected.

## Communities (27 total, 4 thin omitted)

### Community 0 - "testing.T"
Cohesion: 0.08
Nodes (53): TestBenchgenCLI(), TestBenchgenCLIImplFlagRepeatable(), TestBenchgenCLIImplFlagValidation(), TestBenchgenCLIInterfaceParameter(), TestBenchgenCLIRequiredFlags(), invoke(), TestCLIRejectsInvalidInputs(), TestEscapeCLI() (+45 more)

### Community 1 - "placeholder_test.go"
Cohesion: 0.13
Nodes (23): generateCode(), renderCallArguments(), supportedInputType(), TestFloatNaNGenerationWithMathImport(), TestGenerateCodeForTuple(), TestGenerateRejectsWrongTupleArity(), TestRenderCallArgumentsKeepsTwoDigitPlaceholdersDistinct(), TestStringContainingMathDotSurvivesGeneration() (+15 more)

### Community 2 - "profadvisor"
Cohesion: 0.06
Nodes (34): Caveats on contention profiles, Choosing the objective, Commands, I/O contract, profadvisor, `profadvisor apply <patch.diff>`, `profadvisor benchgen --dir <repo> --pkg <package> --func <name> --corpus <directory> --out <directory> [--write] [--impl Interface=Type ...]`, `profadvisor capture --pkg <pattern> [--dir <repo>] [--profile cpu|memory|block|mutex] [--bench <regexp>] [--count N] [--rate N]` (+26 more)

### Community 3 - "FromReaders"
Cohesion: 0.16
Nodes (24): io.Reader, BenchComparison, VerifyResult, compare(), finitePValue(), FromFiles(), FromReaders(), readBenchmarks() (+16 more)

### Community 4 - "Resolve"
Cohesion: 0.16
Nodes (17): Options, Result, profileFile(), profileFlags(), Run(), TestProfileFlagsPerKind(), TestRunAgainstFixtureModule(), TestRunReportsNoMatchingBenchmarks() (+9 more)

### Community 5 - "newRootCmd"
Cohesion: 0.16
Nodes (14): newApplyCmd(), newBenchgenCmd(), newCaptureCmd(), newEscapeCmd(), newExtractCmd(), measurementFlags(), newPromptCmd(), emit() (+6 more)

### Community 6 - "FromProfile"
Cohesion: 0.21
Nodes (23): functionStats, Options, github.com/google/pprof/profile.Function, github.com/google/pprof/profile.Line, github.com/google/pprof/profile.Profile, github.com/google/pprof/profile.Sample, attribute(), attributeFromFocus() (+15 more)

### Community 7 - "Run"
Cohesion: 0.25
Nodes (22): Options, context.Context, changedFiles(), git(), gitError(), nextBranch(), numstat(), rollback() (+14 more)

### Community 8 - "resolveTarget"
Cohesion: 0.17
Nodes (20): listedPackage, go/ast.File, go/types.Interface, go/types.Named, go/types.Package, go/types.Type, collectPackageLevelNames(), flattenFuzzArgument() (+12 more)

### Community 13 - "Run"
Cohesion: 0.11
Nodes (26): Options, Result, Options, time.Duration, Run(), TestRunCancellationKillsRunningTestProcess(), Run(), corpusDir() (+18 more)

### Community 14 - "Generate"
Cohesion: 0.14
Nodes (24): Manifest, Options, Seed, Target, Options, Result, inertCopy(), TestArtifactsCollisionAndInstall() (+16 more)

### Community 15 - "os/exec.Cmd"
Cohesion: 0.40
Nodes (3): os/exec.Cmd, Configure(), Configure()

### Community 16 - "Parse"
Cohesion: 0.12
Nodes (34): position, Result, versionTuple, checkVersion(), hasAnyPrefix(), isExplanationHeading(), isFlowContinuation(), isIgnoredDiagnostic() (+26 more)

### Community 26 - "testing.B"
Cohesion: 0.17
Nodes (14): testing.B, BenchmarkParse(), loadFixtureContent(), rewriteFilesInFixture(), BenchmarkFromProfile(), Table, matches(), New() (+6 more)

### Community 27 - "Text"
Cohesion: 0.07
Nodes (44): Catalog, strings.Builder, Coster(), Dec1(), Nanos(), Pct(), TestTrimShape(), TrimShape() (+36 more)

### Community 28 - "corpus.go"
Cohesion: 0.17
Nodes (23): go/ast.CallExpr, go/ast.Expr, go/token.Token, boolLiteral(), byteSliceLiteral(), charLiteral(), corpusLiteral(), decodeCorpusSeed() (+15 more)

## Knowledge Gaps
- **34 isolated node(s):** `github.com/joaolaureano/profadvisor`, `listedPackage`, `position`, `example.com/escapecorpus`, `example.com/profadvisor/fixture` (+29 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FromProfile()` connect `FromProfile` to `testing.T`, `testing.B`, `Text`, `Resolve`?**
  _High betweenness centrality (0.075) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `testing.B`, `Text`, `Run`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Why does `TestPromptsMatchGolden()` connect `testing.T` to `Text`, `Resolve`, `FromProfile`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._
- **Are the 13 inferred relationships involving `Parse()` (e.g. with `Run()` and `BenchmarkParse()`) actually correct?**
  _`Parse()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **Are the 5 inferred relationships involving `FromProfile()` (e.g. with `BenchmarkFromProfile()` and `load()`) actually correct?**
  _`FromProfile()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `Resolve()` (e.g. with `TestContentionObjectives()` and `TestDefaultsAndInvalidCombinations()`) actually correct?**
  _`Resolve()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `TestAppliesOnNewBranch()` and `TestApplyIgnoresUnrelatedUntrackedFiles()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._