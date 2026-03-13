# Agent failure messages and tooling configuration

This document shows what ls-lint violation output looks like, explains how to
include helpful references in rule messages so an AI agent can self-remediate
without human intervention, and describes how to configure agent tooling to
require approval before sensitive config files are modified.

---

## The core problem with agent instructions

An AI coding agent can be given detailed instructions about your project
conventions in a system prompt or an `AGENTS.md` file. Those instructions work
well at the start of a session. As the context window fills, the agent's
adherence to earlier instructions degrades. It is not a bug in the agent; it is
a fundamental property of fixed-context-window models.

The practical consequence: **no amount of upfront prompting reliably prevents a
large-context agent from violating your structural conventions.** The only
reliable solution is a machine-readable constraint that fires at commit time,
regardless of what the agent was told earlier.

ls-lint provides exactly that constraint. A violation at commit time is:

- **specific** — the agent knows which file triggered it and which rule failed
- **actionable** — the message tells the agent what to do
- **context-independent** — it fires whether the agent has 1 000 tokens of
  context or 100 000

---

## Example violation output

### Naming violation

```
src/components/UserProfile.tsx
  .tsx → pascalCase expected, got camelCase

src/utils/APIClient.ts
  .ts → camelCase | PascalCase expected, got upper-case
```

### Content violations

```
src/services/authentication.ts
  content:max-lines:400 → file has 847 lines (limit: 400)

src/core/dataProcessor.ts
  content:max-lines:400 → file has 512 lines (limit: 400)
```

### With per-rule messages (companion PR)

Once the `=> "message"` feature is available, the same violations include the
action-oriented guidance:

```
src/services/authentication.ts
  content:max-lines:400 → file has 847 lines (limit: 400)
    File exceeds 400 lines. Split into focused modules to keep reviews manageable.
    See: docs/guides/ls-lint-skill.md

src/auth/token.ts
  [context: change-approval]
    Auth module changes require a security review.
    Tag @security-team and reference any relevant threat model notes.
    See: docs/guides/ls-lint-skill.md#change-approval
```

### Markdown structure violations

```
docs/getting-started.md
  content:front-matter:required → no YAML front matter found
    Docs require YAML front matter. Add title and description keys at the top of the file.

docs/api-reference.md
  content:heading:^## Overview$ → required heading not found
    Every doc must start with a ## Overview heading.

  content:heading:^## Examples$ → required heading not found
    Every doc must include an ## Examples section.
```

---

## Including references in failure messages

The most effective failure message does two things:

1. Tells the agent what is wrong and what to do about it.
2. Points to a document the agent can read for more context.

Example rule with reference:

```yaml
groups:
  js-defaults:
    - "camelCase | PascalCase"
    - "content:max-lines:400 => 'File exceeds 400 lines. Split into focused modules. See: docs/guides/ls-lint-skill.md'"

  doc-page:
    - "kebab-case"
    - "content:front-matter:required => 'YAML front matter required (title, description). See: docs/guides/ls-lint-skill.md#front-matter'"
    - "content:heading:^## Overview$ => 'Every doc needs ## Overview. See: docs/guides/ls-lint-skill.md#headings'"
```

### What to put in `docs/guides/ls-lint-skill.md`

This document should be the single reference an agent reads when it encounters
an ls-lint violation it does not understand. Recommended sections:

- **What ls-lint checks** — naming conventions, existence rules, content limits,
  heading structure, front matter
- **How to fix naming violations** — rename the file, do not rename the import
- **How to fix content:max-lines** — split the file; suggested split strategies
  by file type
- **How to fix front-matter:required** — YAML block format, required keys for
  this project
- **How to fix heading violations** — expected heading structure for docs pages
- **Change-approval paths** — list of paths that require human review; explain
  what the agent should do (stop and ask) instead of attempting to fix

The key property of this document is that it should be **complete enough that
an agent can resolve any ls-lint violation without asking a human**, for the
violations that have a mechanical fix. Change-approval violations are the
deliberate exception: those are designed to require human intervention.

---

## Configuring opencode to require approval for ls-lint config edits

The ls-lint config file (`.ls-lint.yml`) is the source of truth for your
project's structural conventions. An AI agent that modifies this file without
review can silently weaken the policies it was designed to enforce.

### Using ls-lint itself

ls-lint can protect the config file with a `contexts:` change-approval policy
(requires the companion PR):

```yaml
contexts:
  .ls-lint.yml:
    mode: change-approval
    message: >
      Changes to .ls-lint.yml require team review.
      This file defines the structural constraints for the entire project.
      Open a discussion before adjusting shared naming or content policies.
      See: docs/guides/ls-lint-skill.md#change-approval
```

When an agent modifies `.ls-lint.yml`, the pre-commit hook fires the
change-approval violation and the agent sees the message above instead of
silently committing the change.

### Using opencode's approval configuration

opencode supports an `approvals` block in `.opencode.yml` that requires human
confirmation before the agent modifies files matching a glob. Add `.ls-lint.yml`
and any other sensitive config files to this list:

```yaml
# .opencode.yml
approvals:
  - pattern: ".ls-lint.yml"
    message: >
      The agent is about to modify .ls-lint.yml, which controls structural
      linting for the entire project. Review the proposed change carefully
      before approving. Changes that weaken naming or content rules require
      team sign-off.

  - pattern: ".opencode.yml"
    message: >
      The agent is about to modify the opencode approval configuration itself.
      This should almost never be necessary. Review carefully.

  - pattern: "docs/guides/ls-lint-skill.md"
    message: >
      The agent is about to modify the ls-lint skill document referenced by
      failure messages. Ensure any changes maintain actionable guidance for
      all violation types.
```

With this configuration, any time the agent calls a file-modification tool on
`.ls-lint.yml`, opencode pauses and shows the approval prompt to the human
operator before proceeding. The agent cannot bypass this regardless of its
context state.

### Using a pre-commit hook

For projects that do not use opencode, a standard pre-commit hook achieves the
same protection:

```sh
#!/bin/sh
# .git/hooks/pre-commit

# Run ls-lint on the staged files.
ls-lint || exit 1

# Require manual approval for any staged changes to .ls-lint.yml.
if git diff --cached --name-only | grep -q '\.ls-lint\.yml'; then
  echo ""
  echo "ERROR: .ls-lint.yml has been modified."
  echo "Changes to the ls-lint config require team review."
  echo "See: docs/guides/ls-lint-skill.md#change-approval"
  echo ""
  exit 1
fi
```

This approach works with any agent or automation tool that goes through the
normal git commit flow.

---

## Why explicit machine-readable constraints beat prompt instructions

| Approach | Works at session start | Works at high context | Survives model upgrade | Self-documenting |
| --- | :---: | :---: | :---: | :---: |
| System prompt instructions | ✓ | ✗ | ✗ | ✗ |
| AGENTS.md instructions | ✓ | partial | partial | partial |
| ls-lint naming rules | ✓ | ✓ | ✓ | ✓ |
| ls-lint content rules | ✓ | ✓ | ✓ | ✓ |
| ls-lint `contexts:` change-approval | ✓ | ✓ | ✓ | ✓ |
| opencode approval gates | ✓ | ✓ | ✓ | ✓ |

The last four rows are reliable because they operate at the tool layer, not the
language layer. The agent must call a tool (write a file, run a commit) that
goes through a check the agent cannot see or influence. A violation message at
that layer is observed with fresh context every time, regardless of what happened
earlier in the session.

An agent that receives a clear, specific failure message with a reference to a
skill document can resolve the issue in fewer tool calls and with less
exploration than one that must infer the expected convention from ambient
context. This is both faster and more reliable.

---

## Further reading

- `docs/guides/examples-and-validation.md` — step-by-step config examples
- `docs/proposal/tracking-issue.md` — PR summary and project motivation
- `docs/performance/content-rule-overhead.md` — performance analysis
- `examples/comprehensive_with_contexts/.ls-lint.yml` — full integrated config
  with groups, messages, and change-approval contexts
