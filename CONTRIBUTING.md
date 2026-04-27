# Contributing

Read `AGENTS.md` first. It is the shared contract for humans and agents alike.

## Setup

```
git clone https://github.com/csellis/ocp.git
cd ocp
make all
```

`make all` runs gofmt, vet, golangci-lint, tests, and a build. Install golangci-lint v2 first: `brew install golangci-lint`. Go 1.25 or newer.

## Workflow

1. Open or claim an issue before non-trivial work.
2. One logical change per PR. Conventional commit format: `type(scope): subject`.
3. Tests live next to code. Table-driven where natural.
4. New domain term? Add it to `.ocp/glossary.md` in the same PR (`AGENTS.md` rule).
5. New dependency? Justify it in the PR description. Standard library is the default.
6. `make all` must pass. CI runs the same set.

## Voice

Direct. Technical. No filler. No emoji in code, commits, issues, or PR comments. No em dashes. See `AGENTS.md` for the full rule set.

## OCP-filed issues

Issues filed by OCP itself carry the `ocp` label. Humans should not use that label. Replies to OCP-authored issues should be terse and substantive.

## Agentic-coding files

`AGENTS.md` is the project contract for humans and AI assistants. `CLAUDE.md` is Claude-specific guidance layered on top. Both are committed. `CLAUDE.local.md` is gitignored and personal.
