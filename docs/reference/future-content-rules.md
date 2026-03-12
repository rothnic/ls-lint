# Future content-rule notation

This note captures a possible direction for a later PR. It is design-only and is
not implemented today.

## Goals

- keep project structure and high-level organization rules in one config
- avoid changing the current `=>` feedback syntax later
- compose content checks with the existing ` | ` rule separator
- support simple inline checks and more reusable named policies
- avoid arbitrary command execution as a validation mechanism

## Stable envelope

Keep the existing rule grammar:

```text
<rule>[:params] => <custom message>
```

Future content checks can fit inside a single `content` rule:

```yaml
ls:
  docs/guides:
    .md: kebab-case | content:max-lines:250 | content:front-matter:required
```

That is compatible with the current parser because rule definitions are split on
the first `:` only. So `content:max-lines:250` is read as:

- rule name: `content`
- rule parameters: `max-lines:250`

This avoids conflicts with existing behavior:

- `=>` still remains the custom-message separator
- ` | ` still remains the way to compose multiple checks
- current releases continue to reject unknown rules until a `content` rule is
  implemented

## Inline checks

Small checks can stay inline:

```yaml
ls:
  docs/guides:
    .md: kebab-case | content:max-lines:250 | content:max-line-length:120
  packages/*:
    README.md: content:heading:^## Overview$
```

This is a good fit for constraints such as:

- total line count
- maximum line length
- required heading patterns
- simple “front matter must exist”
- simple “file must contain a pattern related to its path or basename”

## Named profiles for verbose checks

Once checks need more structure, the inline form should be able to reference a
named profile instead of forcing every rule to encode everything in one string.

One possible shape is:

```yaml
ls:
  docs/guides:
    .md: kebab-case | content:profile:docs_markdown
  components/*:
    .tsx: PascalCase | content:profile:react_component_file

content_checks:
  docs_markdown:
    max-lines: 250
    max-line-length: 120
    front-matter:
      required:
        - title
        - summary
    headings:
      - ^## Overview$
      - ^## Usage$

  react_component_file:
    max-lines: 300
    component:
      count: 1
      name: ${parent_pascal}
```

This keeps common cases short while leaving room for:

- required front matter keys or values
- multiple content checks applied together
- path-aware assertions such as `${parent_pascal}` or `${basename}`
- higher-level file organization checks like “exactly one component”

## Organization templates vs specialized validators

The main boundary should be “catch gross mismatches cheaply” rather than “fully
understand every language.”

For example, a React-oriented profile can reasonably express broad
organization-level expectations such as:

- the file should not exceed a line budget
- the file should contain a recognizable export or component name pattern
- the expected name can be derived from `${basename}` or `${parent_pascal}`

But “find the actual component definition and prove it is the only top-level
React component” quickly becomes AST-aware language analysis. That is a better
fit for a specialized validator than a first-party ls-lint built-in check.

So the practical split should be:

- **good built-ins for ls-lint**: max lines, max line length, front matter
  presence, required headings, simple regex/template checks against file text,
  and lightweight path-aware organization rules
- **better left external**: schema validation, framework-specific AST checks,
  import graph rules, or anything requiring deep semantic understanding of a
  language

## Extensibility model

Extensibility should support two paths:

1. **cheap built-in checks** for common cross-project organization rules
2. **paired external validators** for specialized or language-aware checks

That means ls-lint does not need to implement every possible validator itself.
It can cover the common organization cases directly, while teams keep richer
validation in dedicated tools that run in the same workflow.

The extension point should be new built-in checks or named validator IDs handled
by ls-lint itself, not arbitrary shell commands. That keeps configs portable,
keeps evaluation deterministic, and avoids security issues from executing
repository-defined commands during linting.

In practice, that means future growth can happen by teaching the `content` rule
new built-in check kinds and letting profiles compose them, instead of trying to
anticipate every use case in the first implementation.

For example, schema-level front matter validation can stay outside ls-lint:

```sh
ls-lint --context pre-push && node ./scripts/validate-frontmatter-schema.mjs
```

That still gives one hook entry point and one enforcement stage, while keeping
ls-lint focused on repository structure plus lightweight organization checks.

## Template-oriented checks

If the main goal is outline-level enforcement, a later PR could reserve a small
set of generic content checks aimed at templates rather than full parsers. For
example:

- required headings in a markdown file
- required front matter keys
- ordered section presence
- “must contain a pattern related to the file or folder name”

That is enough to catch the first and second layer problems without promising
100% semantic compliance.

## Performance guidance

Content checks should remain opt-in and run only after a file already matches
the existing path/name rules. That keeps ls-lint’s fast repository walk as the
first filter and limits file reads to the subset that actually needs them.

Good candidates for ls-lint are checks that are:

- local to one file
- cheap to compute
- deterministic
- independent of full project compilation or type analysis

Poor candidates are checks that require:

- parsing entire dependency graphs
- language servers, compilers, or type checkers
- network access
- executing repository-defined commands during ls-lint itself

## Scope boundary

This direction is still intended for the first two layers only:

- **structure**: where files live and what they are named
- **organization**: what broad patterns or required sections matching files must contain

Language-specific formatting and deep semantic linting should remain in
specialized tools.
