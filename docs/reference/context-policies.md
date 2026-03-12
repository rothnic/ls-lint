# Context policies

`contexts:` can stay as a simple string, or it can use a structured form when
you want the config to carry more of the stage policy directly.

## Scalar form

```yaml
contexts:
  pre-commit: Commit is not blocked. Treat these failures as warnings.
```

## Structured form

```yaml
contexts:
  pre-commit:
    mode: warn
    hook: pre-commit
    environment: local
    override: repository owner approval
    change-approval: repository owner approval
    references:
      - docs/contributing.md
      - docs/reference/repo-shape.md
    message: >
      Treat these failures as early warnings about repository structure and
      naming so they can be fixed before push.
```

Optional fields:

- `message`: free-form summary or longer guidance
- `mode`: `warn` or `fail`
- `hook`: associated git hook or enforcement stage
- `environment`: local, CI, pre-merge, or any other short label
- `override`: who can approve bypassing the context
- `change-approval`: who can approve changes to the ls-lint policy itself
- `references`: extra docs or policy links to surface in the formatted output

When the structured form is used, ls-lint builds one readable context message
from the populated fields and prepends it to each failing rule output.

`mode: warn` also lets `ls-lint --context <name>` behave like `--warn` by
default, so hook commands do not need to repeat that policy. Explicit
`--warn` or `--warn=false` still wins if you need to override the selected
context.

## Example formatted output

Text output keeps the normal `path failed for \`rule-key\` rules:` shape and
prepends the rendered context policy once before the failing rule messages.

With this config:

```yaml
ls:
  .png: snake_case => PNG files must use snake_case before pushing
  packages/*:
    README.md: exists:1 => Each package must include README.md before merging

contexts:
  pre-commit:
    mode: warn
    hook: pre-commit
    environment: local
    override: repository owner approval
    change-approval: repository owner approval
    references:
      - docs/reference/context-policies.md
    message: >
      Treat these failures as early warnings about repository structure and
      naming so they can be fixed before push.
```

A filename rule failure looks like:

```text
not-snake-case.png failed for `.png` rules: Context `pre-commit`: warning, pre-commit hook, local environment. Override approval: repository owner approval. Change approval: repository owner approval. References: docs/reference/context-policies.md. Treat these failures as early warnings about repository structure and naming so they can be fixed before push. | PNG files must use snake_case before pushing
```

A directory-scoped `exists` failure keeps the same context prefix and then adds
the underlying rule details:

```text
packages/example failed for `README.md` rules: Context `pre-commit`: warning, pre-commit hook, local environment. Override approval: repository owner approval. Change approval: repository owner approval. References: docs/reference/context-policies.md. Treat these failures as early warnings about repository structure and naming so they can be fixed before push. | exists:1-32767 (found 0) | Each package must include README.md before merging
```

## Hook and CI examples

With `mode` embedded in the context, a hook can stay small:

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

The same pattern works in CI:

```sh
ls-lint --context pre-merge
```

This keeps the stage policy in `.ls-lint.yml` while hook/CI config only selects
which context to apply.

The recommended hook shape is intentionally just `ls-lint --context <name>`.
That path is covered by tests through the same context parsing, message
formatting, and warn/block resolution behavior used by the CLI.

## Agent guardrails

If you want stronger protection around `.ls-lint.yml` itself, keep that
enforcement outside ls-lint and layer it with repository controls such as:

- protected branches and required reviews
- `CODEOWNERS` on `.ls-lint.yml` and hook configuration
- agent-specific safety wrappers or policy tools that treat ls-lint policy files
  as protected inputs
