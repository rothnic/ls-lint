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
    policy-changes: repository owner approval
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
- `policy-changes`: who can approve changes to the ls-lint policy itself
- `references`: extra docs or policy links to surface in the formatted output

When the structured form is used, ls-lint builds one readable context message
from the populated fields and prepends it to each failing rule output.

`mode: warn` also lets `ls-lint --context <name>` behave like `--warn` by
default, so hook commands do not need to repeat that policy. Explicit
`--warn` or `--warn=false` still wins if you need to override the selected
context.

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

## Agent guardrails

If you want stronger protection around `.ls-lint.yml` itself, keep that
enforcement outside ls-lint and layer it with repository controls such as:

- protected branches and required reviews
- `CODEOWNERS` on `.ls-lint.yml` and hook configuration
- agent-specific safety wrappers or policy tools that treat ls-lint policy files
  as protected inputs
