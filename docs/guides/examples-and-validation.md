# ls-lint: examples and validation walkthrough

This guide builds from the simplest possible ls-lint config up to the full
integrated syntax that combines content rules, reusable groups, per-rule
messages, and change-approval contexts. Each step is self-contained and can be
adopted incrementally.

---

## Step 1 — Naming rules only

This is the baseline every project starts with. It adds no runtime overhead
beyond directory traversal and string matching.

```yaml
# .ls-lint.yml
ls:
  .dir: kebab-case
  .ts: camelCase | PascalCase
  .tsx: camelCase | PascalCase
  .js: camelCase | PascalCase
  .jsx: camelCase | PascalCase
  .md: kebab-case

  packages/*:
    .dir: kebab-case
    AGENTS.md: exists:1
    README.md: exists:1
    src: exists:1

ignore:
  - node_modules
  - dist
  - coverage
```

This already eliminates an entire class of review comment: "can you rename that
file to camelCase?" never needs to be written again.

---

## Step 2 — Add inline content rules

Content rules are added inline using the same `|` syntax as naming rules. The
most common starting point is `content:max-lines` to catch files that have
grown too large to review comfortably.

```yaml
# .ls-lint.yml
ls:
  .ts: camelCase | PascalCase | content:max-lines:400
  .tsx: camelCase | PascalCase | content:max-lines:400
  .js: camelCase | PascalCase | content:max-lines:400
  .jsx: camelCase | PascalCase | content:max-lines:400

  # Docs: naming + size + required structure
  docs:
    .md: kebab-case | content:max-lines:250 | content:front-matter:required

  # Generated code may legitimately be larger
  generated:
    .ts: camelCase | PascalCase | content:max-lines:800
    .js: camelCase | PascalCase | content:max-lines:800

  # Vendor code: naming only, no size constraint
  vendor:
    .ts: camelCase | PascalCase
    .js: camelCase | PascalCase

ignore:
  - node_modules
  - dist
  - coverage
```

See `examples/simple_content_rules/.ls-lint.yml` for a runnable version.

### What the first content rule costs

Adding `content:max-lines:400` to all TypeScript files means ls-lint now reads
those files. The benchmarked overhead on representative real-world projects:

| Project | TS/TSX files | naming only | + `content:max-lines:400` | overhead |
| --- | ---: | ---: | ---: | ---: |
| bulletproof-react | ~128 | 9 ms | 15 ms | +67% |
| vite | ~546 | 10 ms | 16 ms | +60% |
| nuxt | ~600 | 15 ms | 26 ms | +73% |

The absolute numbers are small (low tens of milliseconds even for the largest
project). The overhead is significant in relative terms only because the
naming-only baseline is so fast. See
`docs/performance/content-rule-overhead.md` for full benchmark data and
methodology.

---

## Step 3 — Add required markdown headings and front matter

Documentation pages can be required to contain specific headings in a defined
order and to have YAML front matter:

```yaml
ls:
  docs:
    .md: >-
      kebab-case
      | content:front-matter:required
      | content:max-lines:300
      | content:heading:^## Overview$
      | content:heading:^## Usage$
      | content:heading:^## Examples$

  docs/reference:
    .md: >-
      kebab-case
      | content:front-matter:required
      | content:max-lines:600
      | content:heading:^## Overview$
      | content:heading:^## Examples$
```

Each `content:heading` rule is independent: if `## Overview` is present but
`## Usage` is missing, you get a violation specifically for `## Usage`. Heading
patterns use Go regex syntax and are matched against the entire line (including
the `##` prefix and trailing text), so `^## Overview$` requires an exact match.

`content:front-matter:required` checks that the file opens with `---`, is
followed by at least one non-empty line, and closes with another `---` before
any body content begins. It does not validate the keys inside the block.

See `examples/markdown_structure/.ls-lint.yml` for a standalone example.

---

## Step 4 — Extract shared policies into reusable groups

Once the same content rule appears on three or more extension keys, the config
becomes tedious to maintain. Reusable groups solve this: define the policy once
and reference it with `@name`.

```yaml
# .ls-lint.yml
groups:
  # Naming rule shared by all code groups.
  js-names: "camelCase | PascalCase"

  # Standard application code: named correctly and short enough to review.
  js-defaults:
    - "@js-names"
    - "content:max-lines:400"

  # Larger limit for generated or data-heavy files.
  js-large:
    - "@js-names"
    - "content:max-lines:800"

  # Documentation pages require structure.
  doc-page:
    - "kebab-case"
    - "content:front-matter:required"
    - "content:max-lines:300"
    - "content:heading:^## Overview$"
    - "content:heading:^## Usage$"

ls:
  .ts: "@js-defaults"
  .tsx: "@js-defaults"
  .js: "@js-defaults"
  .jsx: "@js-defaults"

  docs:
    .md: "@doc-page"

  generated:
    .ts: "@js-large"
    .js: "@js-large"

  vendor:
    .ts: "@js-names"
    .js: "@js-names"

ignore:
  - node_modules
  - dist
  - coverage
```

Groups can reference other groups (`@js-names` inside `js-defaults`), so
naming conventions stay DRY across all variants. More specific path blocks
replace the parent scope entirely, so to override only the content rule in a
subtree, point that subtree at a different group.

See `examples/reusable_content_rule_sets/.ls-lint.yml` for a fuller example
with content-only overrides and a path that drops the line limit entirely.

---

## Step 5 — Add per-rule failure messages

> **Note:** This syntax requires the companion `feature/per-rule-failure-messages`
> PR to be merged. Remove the `=> "..."` annotations and the rest works today.

Each rule can carry an action-oriented `=> "message"` annotation that appears
in the violation output. This is particularly important for AI agents, which
can act on a specific instruction rather than guessing the expected remediation.

```yaml
groups:
  js-defaults:
    - "@js-names"
    - "content:max-lines:400 => 'File exceeds 400 lines. Split into focused modules to keep reviews manageable.'"

  doc-page:
    - "kebab-case"
    - "content:front-matter:required => 'Docs require YAML front matter. Add title and description keys at the top of the file.'"
    - "content:heading:^## Overview$ => 'Every doc must start with a ## Overview heading.'"
    - "content:heading:^## Usage$ => 'Every doc must include a ## Usage section.'"
```

See `examples/groups_with_messages/.ls-lint.yml` for a complete config using
this syntax.

---

## Step 6 — Add change-approval contexts for sensitive paths

> **Note:** This syntax requires the companion `feature/contexts-change-approval`
> PR to be merged.

The `contexts:` block attaches a change-approval policy to a path glob. When a
matched file is modified, the violation message tells the committer exactly what
is required before the change can be accepted.

```yaml
contexts:
  src/auth:
    mode: change-approval
    environment: production
    message: >
      Auth module changes require a security review.
      Tag @security-team and reference any relevant threat model notes.

  src/api:
    mode: change-approval
    message: >
      API contract changes must be reviewed for backwards compatibility.
      Tag @api-owners and link the relevant RFC before merging.

  .ls-lint.yml:
    mode: change-approval
    message: >
      Changes to the ls-lint config require team review.
      Open a discussion before adjusting shared naming or content policies.
```

See `examples/comprehensive_with_contexts/.ls-lint.yml` for the full integrated
config combining all six steps.

---

## Validation summary

### Synthetic benchmarks (in-repo fixtures)

Run with:

```sh
go test -run '^$' -bench 'BenchmarkLinterRun(ContentScenarios|CodeScenarios)$' \
  -benchmem -count=3 ./internal/linter
```

| Shape | naming only | + `content:max-lines:400` | overhead | + `max-line-length:120` | overhead |
| --- | ---: | ---: | ---: | ---: | ---: |
| 10 TS files | 233.8 µs | 275.4 µs | +17.8% | 297.1 µs | +27.1% |
| 500 TS files | 4.41 ms | 5.49 ms | +24.6% | 5.91 ms | +34.0% |
| 5 000 TS files | 43.67 ms | 58.65 ms | +34.3% | 62.52 ms | +43.2% |

### Real-world project validation

Run with `.github/workflows/real-world-perf.yml` (manual dispatch or weekly):

| Project | TS/TSX files | naming only | + `max-lines:400` | + `max-lines` + `max-line-length` |
| --- | ---: | ---: | ---: | ---: |
| bulletproof-react | ~128 | 9 ms | 15 ms | 17 ms |
| vite | ~546 | 10 ms | 16 ms | 17 ms |
| nuxt | ~600 | 15 ms | 26 ms | 27 ms |

### Violations found in real codebases

Running `content:max-lines:400` against these projects found genuine structural
concerns:

- **vite**: 37 files over 400 lines, including `config.ts` (2,703 lines) and
  `build.ts` (1,922 lines).
- **nuxt**: 20 files over 400 lines, including `nuxt.ts` (1,095 lines).

These are the kinds of large, central files where AI agents will attempt to
make multiple edits in a single session, and where an early warning about file
size pays off.

---

## Further reading

- `docs/performance/content-rule-overhead.md` — benchmark methodology and
  detailed timing tables
- `docs/proposal/tracking-issue.md` — PR summary and motivation
- `docs/guides/agent-failure-messages.md` — what failure output looks like and
  how to configure tooling to surface it effectively
