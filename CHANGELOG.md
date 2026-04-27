# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `ocp scan` reads or seeds `.ocp/glossary.md`.
- `ocp drift` walks the working tree and files one observation per `(synonym, canonical)` pair under `.ocp/conversation/`.
- `ocp respond` reads `## Reply` sections on observations and acts on `close: <reason>`, `synonym: <term>`, or `stand by`. Closes gate 3 with a keyword parser; LLM-driven judgment is deferred to v0.2.
- Filesystem `Storage` backend writing under `.ocp/` with atomic temp-file rename.
- Scout cheap-stage detector. Pure Go, no LLM calls. Respects `.gitignore` via `git ls-files`; excludes `.ocp/` always.
- Eval harness with 5-example labeled corpus. `make eval` reports precision and recall on the synonymy task.
- Self-observation log at `.ocp/log.md`, written on meaningful events.
- CI workflow on push and PR (gofmt, vet, golangci-lint, race tests, build, eval).
- Release workflow gated on tag push or `workflow_dispatch`. GoReleaser builds darwin and linux for amd64 and arm64.
- Build-time version metadata threaded through `cobra` `--version` via `-ldflags`.
- `CONTRIBUTING.md`, `SECURITY.md`, issue and PR templates.

[Unreleased]: https://github.com/csellis/ocp/compare/HEAD
