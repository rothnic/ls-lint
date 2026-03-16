# Lefthook Integration Guide

This directory contains an example `lefthook.yml` that wires ls-lint into
your git hooks using the **context approach**.

## Why use contexts instead of raw hooks?

Without contexts, you configure enforcement behavior in two places:

- `lefthook.yml` — which commands run, whether they block commits
- `.ls-lint.yml` — which structural rules apply

With contexts, policy lives in one place. The `contexts:` block in
`.ls-lint.yml` describes what each enforcement stage means, and lefthook
just passes `--context <name>` to select it.

## Setup

### 1. Install lefthook

```bash
# macOS
brew install lefthook

# npm (any project)
npm install --save-dev lefthook

# Go
go install github.com/evilmartians/lefthook@latest
```

### 2. Install ls-lint

```bash
# npm
npm install --save-dev @ls-lint/ls-lint

# macOS
brew install ls-lint

# curl (Linux/macOS)
curl -sL https://raw.githubusercontent.com/ls-lint/install/master/install.sh | sh
```

### 3. Add lefthook.yml to your repository root

Copy `examples/lefthook_integration/lefthook.yml` to your repository root, or
create a `lefthook.yml` with:

```yaml
pre-commit:
  commands:
    ls-lint:
      run: ls-lint --context pre-commit

pre-push:
  commands:
    ls-lint:
      run: ls-lint --context pre-push
```

### 4. Define contexts in your .ls-lint.yml

Add a `contexts:` block to your `.ls-lint.yml`:

```yaml
contexts:
  pre-commit:
    mode: warn
    hook: pre-commit
    environment: local
    message: >
      Commit is not blocked. Treat these failures as warnings about project
      shape and naming requirements, and queue any quick refactor before
      pushing.

  pre-push:
    mode: fail
    hook: pre-push
    environment: local
    override: repository owner approval
    references:
      - docs/contributing.md
    message: >
      Push is blocked until these structure requirements are resolved or
      explicitly approved by the repository owner.
```

### 5. Install the git hooks

```bash
lefthook install
```

That's it. On every commit, ls-lint runs with the `pre-commit` context (warn
mode). On every push, it runs with the `pre-push` context (fail mode).

## How the context output looks

When violations occur in the `pre-push` context, the output looks like:

```
Context `pre-push`: blocking, pre-push hook, local environment. Override approval: repository owner approval. References: docs/contributing.md. Push is blocked until these structure requirements are resolved or explicitly approved by the repository owner.

src/MyFile.ts failed for `.ts` rules:
  camelCase | PascalCase expected, got upper-case
```

The context message appears once before the violations, giving contributors
the information they need to understand the policy and how to proceed.

## Context-only mode

You can also define a catch-all context for cases where ls-lint is run
without a `--context` flag:

```yaml
contexts:
  "":
    mode: fail
    message: >
      Repository structure violations found. Run with --context pre-commit
      for advisory mode or --context pre-push for enforcement mode.
```

## CI integration

For CI, run ls-lint without a context (default fail behavior) or with a
dedicated `ci` context:

```yaml
# .github/workflows/lint.yml
- name: ls-lint
  run: ls-lint --context ci
```

```yaml
# .ls-lint.yml
contexts:
  ci:
    mode: fail
    environment: CI
    message: >
      Structure violations found in CI. Fix before merging.
```

## Per-rule messages and context

Rule-level messages (using `=>`) and context messages work together.
Rule messages explain what went wrong and why. Context messages explain
the enforcement stage and how to proceed.

```yaml
groups:
  ts-defaults:
    - "camelCase | PascalCase => TypeScript files must use camelCase or PascalCase."
    - "content:max-lines:400 => Keep files under 400 lines for easier review."

contexts:
  pre-push:
    mode: fail
    message: >
      Push blocked. Fix the issues above before pushing.
```

This combination gives contributors exactly what they need at the point
of failure: what the rule is, why it exists, and what the enforcement
stage requires.
