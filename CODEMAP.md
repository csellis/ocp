# Code Map

Where things live in this repo. For *why* the design is shaped the way it is, read `docs/ARCHITECTURE.md`. For what ships next, read `docs/PLAN.md`. For the worldview that decides the tiebreakers, read `docs/THESIS.md`.

This file exists to answer "where is X?" in one read. Keep it accurate. When a new package, binary, or top-level file lands, update the corresponding section in the same change.

## Top level

| Path | What it is |
|---|---|
| `cmd/` | CLI entry points. One subdirectory per binary; each has a `main.go`. |
| `internal/` | Module-private packages. The Go compiler refuses to let anyone outside this module import them. |
| `docs/` | Long-form prose: thesis, architecture, plan. |
| `eval/` | Labeled corpus plus runner. `make eval` reports precision/recall on the synonymy task. |
| `Makefile` | Build, test, lint, format targets. `make all` runs the gauntlet. |
| `.golangci.yml` | Lint configuration (golangci-lint v2 schema). |
| `go.mod` | Module manifest. Floor: Go 1.25. |
| `README.md` | Public face of the project. |
| `AGENTS.md` | Project rules for humans and agents. The shared, committed contract. |
| `CLAUDE.md` | Claude-specific instructions, committed. |
| `CLAUDE.local.md` | Maintainer's private Claude instructions. Gitignored. |
| `LICENSE`, `NOTICE` | Apache 2.0 + attribution. |
| `CONTRIBUTING.md` | Contributor workflow. Setup, voice, agentic-coding files. |
| `SECURITY.md` | Vulnerability reporting policy. |
| `CHANGELOG.md` | User-facing change log per Keep a Changelog. |
| `CODEMAP.md` | This file. |
| `.github/` | CI and release workflows; issue and PR templates. |
| `.goreleaser.yaml` | Release-pipeline build matrix and archive shape. |

## Binaries

- `cmd/ocp/main.go` — the only binary. Subcommands present today: `scan`, `drift`, `respond`. The `serve` subcommand lands in later slices (see `docs/PLAN.md`).

## Packages

Present:

- `internal/storage/` — persistent state for one repo.
  - `storage.go` — the `Storage` interface plus the data types it traffics in (`Glossary`, `Term`, `LogEntry`, `IssueRef`, `IssueState`, `IssueStatus`, `RepoID`) and the `ErrNotFound` sentinel.
  - `filesystem.go` — the v0.1 implementation. Reads and writes files under `.ocp/` inside the watched repo. Atomic writes via temp-file plus rename. Glossary parser and `Glossary.Markdown()` serializer live here too.
  - `filesystem_test.go` — round-trip tests, edge cases, the issue-lifecycle test, the file-mode pin (0o644), and `AllIssueRefs` coverage (open + closed enumeration).

- `internal/scout/` — cheap-stage drift detector. Pure Go, zero LLM calls. Walks the working tree for textual occurrences of glossary synonyms; returns `Hit` values for the next stage to judge.
  - `scout.go` — `Detect(ctx, root, glossary) []Hit`. Dispatches between the git-aware path (when root is a git repo) and a plain filesystem walk fallback. Word-boundary regex per synonym, file-extension allowlist (`.go`/`.md`/`.toml`), excludes `.ocp/` always, hidden dirs only in the fallback.
  - `git.go` — `gitTrackedFiles(ctx, root)`. Two short subprocess calls (`git rev-parse`, `git ls-files --cached --others --exclude-standard`) to get the set of paths git considers in-repo. Returns `errNotGitRepo` outside a working tree; caller falls back to walking.
  - `scout_test.go` — detection tests covering matching, multi-word synonyms, word-boundary correctness, dir/extension exclusions, context cancellation, gitignore respect, and the `.ocp/` always-exclude rule.

- `internal/voice/` — observation formatting in OCP voice.
  - `voice.go` — `Format(Body) string` produces the observation markdown. Pure: no I/O, no random, no time. Caller passes the Oblique card and ship-name explicitly.
  - `oblique.go` — `ObliquePack` (curated Eno cards) plus `PickCard()` for random selection per observation.
  - `voice_test.go`, `oblique_test.go` — body-shape and pack-membership coverage.

- `internal/names/` — Banks-style ship-name pack.
  - `names.go` — `Pack` and `Default()`. v0.1 hardcodes the dogfood instance to `Pack[0]`; future slices add per-repo persistence via `.ocp/config.toml` and seed-based selection.

- `eval/` — eval harness and labeled corpus.
  - `main.go` — `package main` runner. Walks `corpus/`, runs `scout.Detect` on each example's `testdata/`, computes precision/recall against `expected.json`, prints a markdown report.
  - `corpus/<NNN-slug>/{testdata/, glossary.md, expected.json}` — one directory per example.
  - `README.md` — corpus shape and how to add an example.

Planned, not yet present (see `docs/PLAN.md` for the build order):

- `internal/agent/` — pi-style stateful agent primitives. Deferred to v0.2 alongside cognition.
- `internal/cognition/` — LLM seam; `vertex/` subpackage wraps Gemini (default model: 2.5 Flash). Deferred to v0.2.
- `internal/tools/` — agent tools: `parse_diff`, `read_glossary`, `find_term_uses`, `github_issues`, etc. Deferred to v0.2.
- `internal/triggers/` — invocation surfaces: `cli`, `webhook` (v0.2), `scheduler` (v0.2).

## Where to look for things by topic

| If you want to ... | Look in |
|---|---|
| Read or write `.ocp/glossary.md` | `internal/storage/filesystem.go` (`LoadGlossary`, `SaveGlossary`) |
| Append to `.ocp/log.md` | `internal/storage/filesystem.go` (`AppendLog`) |
| List or update open observations | `internal/storage/filesystem.go` (`LoadOpenIssues`, `RecordIssueState`) |
| Add a new Storage method | edit the interface in `internal/storage/storage.go`, then update each impl |
| Change the on-disk glossary or observation format | `internal/storage/filesystem.go` (`parseGlossary`, `Glossary.Markdown`, `serializeObservation`) |
| Find synonym occurrences in a tree | `internal/scout/scout.go` (`Detect`) |
| Tune what scout scans (extensions, excluded dirs) | `internal/scout/scout.go` (`isScannable`, `isExcludedDir`, `isAlwaysExcluded`) |
| Change how scout discovers files in a git repo | `internal/scout/git.go` (`gitTrackedFiles`) |
| Tune the seed glossary OCP writes on first run | `cmd/ocp/main.go` (`seedGlossary`) |
| Add a new ocp subcommand | `cmd/ocp/main.go` (declare `*cobra.Command`, register in `init`) |
| Change how drift candidates become observations | `cmd/ocp/main.go` (`groupHits`, `candidateBody`) |
| Change the OCP voice template | `internal/voice/voice.go` (`Format`) |
| Add or change Oblique cards | `internal/voice/oblique.go` (`ObliquePack`) |
| Add or change ship-names | `internal/names/names.go` (`Pack`) |
| Add or change an eval corpus example | `eval/corpus/<NNN-slug>/`; see `eval/README.md` |
| Change the eval runner or report format | `eval/main.go` |
| Change observation filename slug rules | `cmd/ocp/main.go` (`slugify`, `slugFromPath`) |
| Enumerate every observation (open + closed) | `internal/storage/filesystem.go` (`AllIssueRefs`) |
| Change build, test, or lint behavior | `Makefile`, `.golangci.yml` |
| Change agent rules or voice for everyone | `AGENTS.md` |
| Change Claude's per-maintainer behavior | `CLAUDE.local.md` |
| Read the thesis or design rationale | `docs/THESIS.md`, `docs/ARCHITECTURE.md` |
| See what ships next | `docs/PLAN.md` |
| See what is in the maintainer's mind | `docs/todo.md` |

## Dependency direction

Packages in this repo import only downward. Nothing in `internal/` may import anything in `cmd/`. Storage does not import the agent. Cognition does not import tools. The interfaces in `storage` and `cognition` are seams; concrete implementations hide behind them.

```
cmd/ocp ──► internal/triggers ──► internal/agent ──► {cognition, tools, storage}
                                                     │
                                                     ▼
                                          internal/storage
                                                     │
                                                     ▼
                                       filesystem.go (v0.1)
                                       firestore.go (v0.2)
```

Diagram describes the intended graph; today only `cmd/ocp` and `internal/storage` exist.

## Conventions

- Tests live next to the code they test (`foo.go` and `foo_test.go` in the same directory).
- Integration tests requiring a live model use `//go:build integration` and are skipped by `go test ./...`.
- One package per directory. Package name matches the directory name.
- Standard library before third-party. Every dependency justified in the PR description (per `AGENTS.md`).
