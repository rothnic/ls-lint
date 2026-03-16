# Agent Guidance: TypeScript Project Structure Policy

This file is the authoritative reference for code structure constraints in
this TypeScript project. ls-lint enforces the rules defined in `.ls-lint.yml`;
this file explains the rationale, the override process, and how to split files
that violate the size limit.

## File size limits

| Context | Limit | Rationale |
|---|---|---|
| Application source (`.ts`, `.tsx`) | 400 lines | Keeps modules reviewable, composable, and easy to navigate in one mental pass |
| Test files (`.test.ts`, `.spec.ts`) | 600 lines | Test suites need more space but should still stay focused on one concern |
| Generated code (`src/generated/`) | 2000 lines | Machine-generated; human review is a secondary concern |
| Documentation (`.md` in `docs/`) | 500 lines | Keeps each page focused and fast to load for both humans and agents |

## How to analyse a file before splitting

Run the analysis script from the repository root:

```bash
scripts/analyze-ts-structure.sh src/path/to/large-file.ts
```

The script prints:
- Total line count
- All public exports, classes, and top-level functions with line numbers
- Type and interface declarations
- Approximate section boundaries

Use this output to decide where the natural extraction points are. A file
exceeding the limit typically contains one of these patterns that suggests a
clear split:

- Multiple unrelated exported functions → extract into focused utility modules
- A large class with distinct method groups → split into a base class and mixins or extract method groups into helper classes
- A component file with embedded hooks, constants, and helper functions → move helpers to `utils/`, hooks to `hooks/`
- A service file that handles more than one resource type → one file per resource

## How to split a large documentation page

A documentation file that exceeds 500 lines should be converted into a
directory. The original `large-doc.md` becomes:

```
docs/large-doc/
  index.md          # Introduction and overview (links to sections)
  section-one.md    # First major topic
  section-two.md    # Second major topic
  reference.md      # Reference material, tables, examples
```

Each section file should fit within 500 lines. If a section is inherently
long (for example, an exhaustive API reference or a template document), place
it in `docs/large-doc/templates/` — that subdirectory has a relaxed 2000-line
limit and is exempt from the front-matter requirement.

## Override process

**Agents may not approve their own overrides.** The `.ls-lint.yml` file is
the structural contract for this repository. An agent that encounters a size
violation must:

1. Run `scripts/analyze-ts-structure.sh` on the file to understand its shape.
2. Propose a split following the patterns above.
3. If a split is genuinely not possible (rare; must be justified), open a pull
   request describing why the constraint cannot be met. Do not modify
   `.ls-lint.yml` directly.

Overrides to `.ls-lint.yml` require explicit approval from the repository
owner. The pre-push context will block until that approval is recorded.

## Front matter for documentation

All documentation pages in `docs/` require YAML front matter. This enables
navigation index generation, link previews, and reliable context loading when
an agent needs to search the documentation.

Minimum required fields:

```yaml
---
title: Short Page Title
description: One sentence that summarises what this page covers.
---
```

Optional fields that improve discoverability:

```yaml
---
title: Short Page Title
description: One sentence summary.
tags: [api, authentication]
audience: developers
status: draft | review | stable
---
```

Choosing a `description`: write it as you would a pull request summary —
one sentence that tells a reader exactly what they will find without needing
to open the file.
