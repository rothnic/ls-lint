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

## Extension model

The extension point should be new built-in checks or named validator IDs handled
by ls-lint itself, not arbitrary shell commands. That keeps configs portable,
keeps evaluation deterministic, and avoids security issues from executing
repository-defined commands during linting.

In practice, that means future growth can happen by teaching the `content` rule
new built-in check kinds and letting profiles compose them, instead of trying to
anticipate every use case in the first implementation.

## Scope boundary

This direction is still intended for the first two layers only:

- **structure**: where files live and what they are named
- **organization**: what broad patterns or required sections matching files must contain

Language-specific formatting and deep semantic linting should remain in
specialized tools.
