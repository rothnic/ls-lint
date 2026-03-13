# Content Rule Performance Analysis

This document records the follow-up performance analysis for the `content:*`
directive work and explains how the implementation keeps the overhead bounded
when content validation is enabled.

## How the implementation limits overhead

The content-validation path is designed to keep the extra work proportional to
the rules that are actually configured for a file:

1. **Filename-only rules stay filename-only.**
   `internal/linter/linter.go` only calls `fs.ReadFile` when the matched rule
   set contains at least one `content:*` rule.
2. **Matched files are prepared once.**
   After the file is read, `internal/linter/linter.go` combines the
   `PreparedContentOptions` requested by all matched content rules and builds a
   single shared `PreparedContent` instance.
3. **Derived metadata is computed lazily.**
   `internal/rule/content.go` only computes the expensive pieces when a rule
   asks for them:
   - `max-line-length` enables rune-count tracking
   - `front-matter:required` enables front-matter detection
   - `max-lines` and `heading` reuse the normalized line slice without extra
     per-rule preparation

That means the primary transition is still the same one reviewers expected:
moving from naming-only validation into reading file contents. After that, each
additional content rule is comparatively cheaper because it reuses the same
prepared data.

## Benchmark command

All numbers below were collected locally with:

```sh
go test -run '^$' -bench 'BenchmarkLinterRun(ContentScenarios|CodeScenarios)$' -benchmem ./internal/linter
```

Environment:

- `goos: linux`
- `goarch: amd64`
- `cpu: AMD EPYC 7763 64-Core Processor`

## Benchmark shapes

The benchmark suite uses hermetic in-repo fixtures instead of external cloned
repositories so the measurements stay reproducible in local runs and CI.

### Markdown content scenarios

`BenchmarkLinterRunContentScenarios` uses realistic documentation files and
compares:

- `NoContent`: filename rule only
- `SingleContent`: filename rule + `content:max-lines:40`
- `FullContent`: filename rule + `max-lines`, `max-line-length`, `heading`, and
  `front-matter:required`

File-count shapes:

- `FewFiles`: 10 files
- `ModerateFiles`: 500 files
- `ManyFiles`: 5000 files

### TypeScript code scenarios

`BenchmarkLinterRunCodeScenarios` uses a representative TypeScript service file
and applies the rule set to all `.ts` files in a synthetic multi-directory
codebase.

Compared configurations:

- `NamingOnly`: `camelCase | PascalCase`
- `NamingPlusMaxLines`: naming rules + `content:max-lines:400`
- `NamingPlusMaxLinesAndLineLength`: naming rules + `content:max-lines:400 |
  content:max-line-length:120`

This models the review question more directly: what happens when a realistic
line-count or line-length constraint is applied across all code files instead of
only filename rules.

## Results

### Markdown content scenarios

| Shape | NoContent | SingleContent | Overhead | FullContent | Overhead |
| --- | ---: | ---: | ---: | ---: | ---: |
| FewFiles | 82.5 µs/op | 96.5 µs/op | +17.0% | 115.1 µs/op | +39.6% |
| ModerateFiles | 2.53 ms/op | 3.28 ms/op | +29.5% | 3.57 ms/op | +40.7% |
| ManyFiles | 24.42 ms/op | 32.18 ms/op | +31.8% | 35.03 ms/op | +43.4% |

### TypeScript code scenarios

| Shape | NamingOnly | +MaxLines | Overhead | +MaxLines+LineLength | Overhead |
| --- | ---: | ---: | ---: | ---: | ---: |
| FewFiles | 93.9 µs/op | 136.2 µs/op | +45.1% | 143.9 µs/op | +53.3% |
| ModerateFiles | 2.89 ms/op | 4.00 ms/op | +38.3% | 4.31 ms/op | +49.2% |
| ManyFiles | 29.07 ms/op | 40.62 ms/op | +39.8% | 43.56 ms/op | +49.9% |

The code-file benchmark is the more representative result for "naming rules vs
naming rules plus line limits on all code files":

- the largest step is enabling the first content rule, because that forces file
  reads and shared content preparation
- adding `max-line-length` on top of `max-lines` is a smaller incremental step
  than the initial transition into content reads

## Remaining hot spots

The current implementation already removed the most obvious duplication by
preparing content once per file instead of once per rule. Based on the current
code path and measurements, the remaining hot spots are:

1. **File reads for matched files.**
   This is the largest unavoidable cost once any `content:*` rule is enabled.
2. **Normalization and line splitting.**
   `NewPreparedContent` still has to normalize line endings, strip BOMs, and
   split the file into lines once per matched file.
3. **Rune counting for `max-line-length`.**
   This is only done when the rule is configured, and the benchmark shows it is
   incremental rather than dominant.

At this point there is no equally large duplication left to remove with a small
localized change. If future profiling shows more pressure here, the next places
to investigate would be reducing string-allocation work during normalization or
supporting specialized fast paths for single-rule content checks.
