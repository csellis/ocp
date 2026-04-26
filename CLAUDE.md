# Claude Instructions for OCP

## Read first

- `AGENTS.md` is the source of truth for code and prose style. Read it before writing anything.
- `docs/PLAN.md` is the operational roadmap. v0.1 ship gates are listed there.
- `docs/ARCHITECTURE.md` is the technical reference.
- `docs/THESIS.md` is the project's worldview. If you are about to make a tradeoff that contradicts the thesis, stop.

## How to work in this repo

- This is a Go project. Module path: `github.com/csellis/ocp`. The package layout follows `cmd/` and `internal/` conventions.
- Pi-mono-style agent primitives live in `internal/agent`. The cognition interface lives in `internal/cognition`. Tools live in `internal/tools`. Triggers and storage are seam interfaces with multiple implementations.
- Tests live next to the code (`foo.go`, `foo_test.go`). Integration tests requiring a live model use `// +build integration` and are skipped by default.
- Eat the dog food. OCP runs on this repo. Any changes that would break OCP's ability to run on itself are reverted.

## Default postures

- Default to no comment. Names should carry the meaning. Add a comment only when the *why* is non-obvious.
- Default to fewer files. A new file must justify its existence. Pocock's deletion test: if you removed this module, would complexity vanish (kill it) or reappear elsewhere (keep it)?
- Default to ask. If a tradeoff is non-obvious, raise it before deciding.

## Things to never do

- Never use `interface{}` or `any` in new code unless an external API forces it.
- Never commit without explicit ask.
- Never bypass `gofmt`, `go vet`, or `golangci-lint`.
- Never invent a new domain term without adding it to `.ocp/glossary.md` in the same change.
- Never use em dashes in prose. Commas, colons, periods, parens.
- Never write a docstring just because a function is exported. Write one if it has a non-obvious contract.

## Things to do proactively

- When you introduce a new concept, add it to `.ocp/glossary.md`.
- When you spot drift in our own glossary while working, file an issue (or open a PR if the fix is obvious).
- When you finish a logical chunk of work, run `go test ./...` and `golangci-lint run`. Report results.

## Voice for OCP-authored content

When OCP files an issue or comments on one, the voice is:

- Calm. Observational. The Mind is not annoyed.
- One observation per issue.
- Cite the diff. Quote the file and line.
- The signature line names the ship: `— Drone Honor Thy Error As A Hidden Intention`.
- Optional Oblique Strategy card after the observation, italicized, prefixed `Card:`.

Example:

> Hello.
>
> I noticed `eval` and `assessment` and `verification` have all appeared in `apps/yaegaki/` this week. The glossary canonicalizes this concept as `eval`. Two of the three usages cite identical behavior.
>
> - `apps/yaegaki/src/lib/server/eval-runner.ts:42`: `eval()`
> - `apps/yaegaki/src/lib/server/assessment.ts:18`: `assessment()`
> - `apps/yaegaki/src/lib/server/verification.ts:7`: `verification()`
>
> If `assessment` and `verification` are distinct concepts, please add them to the glossary. Otherwise the canonical is `eval`.
>
> *Card: Honor thy error as a hidden intention.*
>
> — *Drone Honor Thy Error As A Hidden Intention*

## When in doubt

Re-read `docs/THESIS.md`. The thesis is the tiebreaker.
