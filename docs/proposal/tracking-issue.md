# ls-lint extended capabilities: tracking issue proposal

## Why these changes

AI coding agents, automated refactors, and multi-contributor monorepos all
share the same failure mode: code that is individually syntactically correct
but structurally wrong. A new component appears in the wrong directory, a
helper grows to 2,000 lines without anyone noticing, or a documentation page
ships without front matter and is therefore invisible to the search index.

Naming rules alone catch the first class of problem. The changes tracked here
extend ls-lint to also enforce file-level structural constraints (size, required
headings, front matter), and to do so in a way that is explicit enough for an
AI agent to interpret without human intervention: clear failure messages,
self-documenting configs, and change-approval policies that fire before a
commit lands.

The key insight that motivates the `contexts:` directive in particular is that
no amount of system-prompt instruction guarantees an agent will follow your
policies as context grows. A machine-readable, version-controlled constraint
that surfaces a violation with a specific remediation message at commit time is
far more reliable than hoping the agent remembered your instructions.

---

## Branches and PRs

### PR 1 — `feature/extend-exists-with-required-rules`

**What it implements**

- Count-based `exists` rule: `exists:0` (forbid), `exists:1` (require exactly
  one), `exists:1-4` (range), and bare `exists` (at least one).
- Explicit basename keys for files and directories, matched before
  extension-pattern keys.
- Full test coverage for count semantics and basename matching.

**What it enables**

Projects can express "every package must have an `AGENTS.md` and a `README.md`"
as a first-class ls-lint rule rather than a CI script. Monorepo scaffolding
constraints become self-documenting.

---

### PR 2 — `copilot/implement-content-directive-validation`

**What it implements**

1. **Content validation rules**: `content:max-lines`, `content:max-line-length`,
   `content:heading`, and `content:front-matter:required`.
2. **Optimised content pipeline**: file is read once per matched file; derived
   metadata (line count, max line length, front matter) is computed lazily and
   shared across all content rules for that file.
3. **Correctness fix**: `content:front-matter:required` now rejects empty or
   whitespace-only front matter blocks.
4. **First-class reusable rule groups**: top-level `groups:`
   namespace so shared rule sets are defined once and referenced with
   `group:<name>` or the compact `@<name>` shorthand. Groups can reuse other
   groups.
5. **Real-world validation**: `bulletproof-react`, `vite`, and `nuxt` were
   measured against naming-only vs content-enabled configs. Results are in
   `docs/performance/content-rule-overhead.md`.
6. **CI workflow**: `.github/workflows/real-world-perf.yml` — runnable on
   `workflow_dispatch` or weekly so the measurements stay anchored to real
   project shapes.

**What it enables**

Teams can set a single policy — "all TypeScript files must be ≤ 400 lines" —
once in `groups:`, reference it with `@js-defaults`, and override it only where
needed. AI agents that create files get immediate, specific feedback when they
violate the policy rather than a generic "too large" comment from a human
reviewer days later.

---

### PR 3 — `feature/per-rule-failure-messages`  *(companion — not yet merged)*

**What it implements**

- `=> "message"` annotation syntax on any rule string.
- Violation output includes the custom message so the reason and remediation
  are visible directly in the lint output rather than requiring a trip to the
  docs.

**What it enables**

Rule authors can attach action-oriented guidance: `"File exceeds 400 lines.
Split into focused modules."` or `"Docs require YAML front matter (title,
description)."`. This is particularly important for AI agents, which can act on
a specific instruction in the error message rather than guessing.

---

### PR 4 — `feature/contexts-change-approval`  *(companion — not yet merged)*

**What it implements**

- `contexts:` top-level block that binds a change-approval policy to a path
  glob.
- Fields: `mode: change-approval`, `environment`, `override`, `references`,
  and `message`.
- Violation output includes the context message when a matched file is modified
  in the configured environment.

**What it enables**

Sensitive paths (`src/auth`, `src/api`, `src/infrastructure`) can be declared
as requiring human review before a change is accepted. An AI agent that edits
`src/auth/token.ts` sees a violation with the message "Auth module changes
require a security review. Tag @security-team." at commit time, before the PR
is even opened.

---

## What they enable together

Once all four PRs land, a single `.ls-lint.yml` file can:

- enforce naming conventions throughout a project tree
- require specific files and directories to exist (`AGENTS.md`, `README.md`,
  `src/`)
- bound file size so growth is caught early
- require documentation pages to have front matter and specific headings in the
  right order
- attach human-readable, action-oriented failure messages to every rule
- declare change-approval policies for sensitive paths that fire at commit time
  with clear guidance for the committer

See `examples/comprehensive_with_contexts/.ls-lint.yml` for a complete config
that uses all four capabilities together.

---

## References

- `docs/guides/examples-and-validation.md` — step-by-step walkthrough from the
  simplest inline config to the full integrated syntax
- `docs/performance/content-rule-overhead.md` — benchmark methodology and
  real-world timing data
- `docs/guides/agent-failure-messages.md` — example failure messages and
  recommended agent tooling configuration
