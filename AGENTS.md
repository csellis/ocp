# Project Rules

These rules apply to humans and agents alike.

## Voice

- Direct. Technical. No filler.
- No emoji in code, commits, issues, PR comments, or docs.
- No marketing language. No "blazingly fast." No "delightful."
- One sentence per claim where possible.
- Plain commas, colons, periods, parens. No em dashes.

## Code

- Go 1.25 minimum.
- Standard library before third-party. Justify every dependency in the PR description.
- `gofmt`, `go vet`, `golangci-lint run` must pass before commit.
- No `interface{}` / `any` unless an external API forces it. Use generics.
- Errors are values. Wrap with `fmt.Errorf("...: %w", err)`. No `panic` in normal control flow.
- Context propagation everywhere. `context.Context` is the first parameter of every IO-touching function.
- Tests use the standard `testing` package, table-driven where applicable.
- Deep modules over shallow modules (Ousterhout). When in doubt, fewer files with bigger interiors.
- A new file must justify its existence. A new package must justify its existence twice.

## Commits

- Conventional commits: `type(scope): subject`.
- One logical change per commit.
- Never commit unless the user (or a maintainer) explicitly asks.
- Never amend. Always new commit.
- Never `--no-verify`, `--force`, `--force-with-lease` without explicit instruction.

## Issues and PRs

- OCP files issues with the label `ocp`. Humans should not use this label.
- OCP-filed issues use the title format `OCP: <one-line observation>`.
- When closing an OCP issue from a commit, include `closes #<number>` in the commit message.
- Comment bodies on OCP issues should be terse. Three sentences or fewer where possible.

## Glossary discipline

OCP runs on its own repo. Therefore:

- Every new domain concept introduced into the codebase must be added to `.ocp/glossary.md` in the same PR that introduces it.
- If you find yourself writing a synonym for an existing canonical term, stop and use the canonical.
- If a canonical term is wrong, update the glossary in its own PR before changing the code.

## When the agent is uncertain

- Ask, do not guess.
- Do not file an issue you cannot defend.
- A "stand by" observation is preferable to a confident wrong observation.

## Sessions

OCP publishes anonymized work-sessions to a public HuggingFace dataset (modeled on `pi-mono`). This is opt-in per repo via `.ocp/config.toml`. See `docs/SESSIONS.md`.
