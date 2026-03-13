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
go test -run '^$' -bench 'BenchmarkLinterRun(ContentScenarios|CodeScenarios)$' -benchmem -count=3 ./internal/linter
```

The tables report the mean of those three runs so the PR body can carry a short,
copyable summary. If you want a stricter statistical comparison between two raw
benchmark captures, collect the `go test` output and run `benchstat` on it, for
example:

```sh
go test -run '^$' -bench BenchmarkLinterRunCodeScenarios -benchmem -count=10 ./internal/linter > /tmp/code-before.txt
go test -run '^$' -bench BenchmarkLinterRunCodeScenarios -benchmem -count=10 ./internal/linter > /tmp/code-after.txt
benchstat /tmp/code-before.txt /tmp/code-after.txt
```

Environment:

- `goos: linux`
- `goarch: amd64`
- `cpu: AMD EPYC 7763 64-Core Processor`

## Benchmark shapes

The benchmark suite uses hermetic in-repo fixtures instead of external cloned
repositories so the measurements stay reproducible in local runs and CI. Each
scenario now also includes ignored repo noise so the timings reflect a more
realistic workspace layout:

- `node_modules`
- `dist`
- `coverage`
- `packages/*/dist`

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
codebase while the ignored paths above are also present in the filesystem.

Compared configurations:

- `NamingOnly`: `camelCase | PascalCase`
- `NamingPlusMaxLines`: naming rules + `content:max-lines:400`
- `NamingPlusMaxLinesAndLineLength`: naming rules + `content:max-lines:400 |
  content:max-line-length:120`

This models the review question more directly: what happens when a realistic
line-count or line-length constraint is applied across all code files instead of
only filename rules.

## Results

### Quick JavaScript/TypeScript timing summary

These are the quickest "what should I expect?" numbers for the PR body. Each
scenario includes the ignored paths above in addition to the checked source
files.

| Modeled project | Checked `.ts` files | NamingOnly | + `content:max-lines:400` | + `content:max-lines:400` + `content:max-line-length:120` |
| --- | ---: | ---: | ---: | ---: |
| Small service | 10 | 233.8 µs/op | 275.4 µs/op | 297.1 µs/op |
| Medium workspace | 500 | 4.41 ms/op | 5.49 ms/op | 5.91 ms/op |
| Large monorepo slice | 5000 | 43.67 ms/op | 58.65 ms/op | 62.52 ms/op |

Relative to naming-only validation, the first content rule is still the biggest
step because it causes file reads. Adding `max-line-length` on top of
`max-lines` is still a measurable extra cost, but it is smaller than the
initial transition from naming-only checks into content reads.

### Markdown content scenarios

| Shape | NoContent | SingleContent | Overhead | FullContent | Overhead |
| --- | ---: | ---: | ---: | ---: | ---: |
| FewFiles | 181.4 µs/op | 189.6 µs/op | +4.5% | 222.1 µs/op | +22.5% |
| ModerateFiles | 3.84 ms/op | 4.49 ms/op | +17.2% | 5.06 ms/op | +32.0% |
| ManyFiles | 38.18 ms/op | 46.39 ms/op | +21.5% | 50.90 ms/op | +33.3% |

### TypeScript code scenarios

| Shape | NamingOnly | +MaxLines | Overhead | +MaxLines+LineLength | Overhead |
| --- | ---: | ---: | ---: | ---: | ---: |
| FewFiles | 233.8 µs/op | 275.4 µs/op | +17.8% | 297.1 µs/op | +27.1% |
| ModerateFiles | 4.41 ms/op | 5.49 ms/op | +24.6% | 5.91 ms/op | +34.0% |
| ManyFiles | 43.67 ms/op | 58.65 ms/op | +34.3% | 62.52 ms/op | +43.2% |

The code-file benchmark is the more representative result for "naming rules vs
naming rules plus line limits on all code files":

- the largest step is enabling the first content rule, because that forces file
  reads and shared content preparation
- adding `max-line-length` on top of `max-lines` is still measurable, but it is
  a smaller incremental step than the initial transition into content reads

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
