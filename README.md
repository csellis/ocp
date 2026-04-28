# OCP

[![CI](https://github.com/csellis/ocp/actions/workflows/ci.yml/badge.svg)](https://github.com/csellis/ocp/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/csellis/ocp.svg)](https://pkg.go.dev/github.com/csellis/ocp)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Outside Context Problem. Open-Closed Principle. Both meanings hold.

OCP is a small agent that watches a codebase for drift in its ubiquitous language: the canonical names a team has agreed on for the concepts in their domain. When a new term appears that conflicts with the glossary, or when one canonical concept is being expressed in three different words, OCP files an issue. The issue is the conversation. The glossary is the memory.

This is not a linter. OCP is an ambient agent. It speaks rarely. It speaks deliberately. When it speaks, the speech-act is a single observation, a citation from the diff, and (sometimes) an Oblique Strategies card. Run as a local CLI, that observation is written to a file in `.ocp/`. Run remotely against a GitHub repo, it is filed as a GitHub Issue.

## The thesis

Matt Pocock argues that AI in a good codebase is excellent and AI in a bad codebase compounds entropy. Bad code is not cheap. Software fundamentals (Ousterhout's deep modules, Evans's ubiquitous language, Beck's "invest in the design of the system every day") matter more in the agentic age, not less.

Pocock's tools (`grill-me`, `ubiquitous-language`, `improve-codebase-architecture`) are one-shot, human-invoked diagnostics. The drift starts again the moment you close the tab.

OCP is a missing piece: the continuous version. One way to operationalize "invest in the design of the system every day," runnable when you remember and schedulable when you stop.

> Bad code is the most expensive it's ever been.
> — Matt Pocock, *Software Fundamentals Matter More Than Ever*

> Invest in the design of the system every day.
> — Kent Beck

## What it does today

OCP runs as a local CLI against a repo on disk. Three subcommands, one loop:

1. **`ocp scan`** finds the team's ubiquitous language file. If `.ocp/glossary.md` is missing, it seeds one from the project's own canonical vocabulary. If present, it reads it and prints. You edit; future runs respect your edits.
2. **`ocp drift`** walks the working tree (git-aware: respects `.gitignore`) for occurrences of words listed as synonyms in the glossary. Each `(synonym, canonical)` pair is filed as one observation under `.ocp/conversation/`, with file and line citations. Idempotent: re-running with no new drift files no new observations.
3. **`ocp respond`** reads `## Reply` sections appended to open observations and acts on `close: <reason>`, `synonym: <term>`, or `stand by`. Closes move to `.ocp/conversation/closed/`; synonym additions land in the glossary; the log records the transition.

OCP runs on its own repo from day one. The first observation in `.ocp/log.md` is OCP noticing OCP.

## What it does not do yet

The roadmap is in `docs/PLAN.md`. The honest gaps in v0.1-dev:

- **No LLM yet.** `respond` is a keyword parser, not a judgment call. Wiring Vertex / Gemini for the conversation loop is the v0.2 work.
- **Synonymy only.** The other two drift modes (ambiguity, one word for many concepts; vagueness, overloaded or imprecise terms) ship with v0.3.
- **Local mode only.** Remote mode against a GitHub repo (issues for observations, comments for replies, PRs for glossary edits, Firestore for cross-repo state) ships with v0.2.
- **No Oblique Strategy cards in observations yet.** The card pack exists; the integration into observation bodies waits until the LLM-driven respond lands.

## Install

Requires Go 1.25 or newer.

```
go install github.com/csellis/ocp/cmd/ocp@latest
```

Or build from source:

```
git clone https://github.com/csellis/ocp.git
cd ocp
make bin/ocp
```

The binary lands at `bin/ocp`. Put it on your `PATH` or invoke directly.

## Usage

Run from inside the repo you want OCP to watch.

`ocp scan` reads or seeds `.ocp/glossary.md` and prints the current glossary:

```
$ ocp scan
wrote new glossary at /your/repo/.ocp/glossary.md (7 terms)

# Glossary

## OCP
...
```

On subsequent runs it reports the term count and prints the existing glossary unchanged.

`ocp drift` walks the working tree for occurrences of glossary synonyms and files one observation per `(synonym, canonical)` pair under `.ocp/conversation/`:

```
$ ocp drift
2 candidates: 2 new (filed), 0 existing
```

`ocp respond` reads any `## Reply` block you appended to an open observation and acts on it:

```
$ ocp respond
1 reply: 1 closed, 0 glossary updates, 0 stand-by
```

Recognized reply intents: `close: <reason>`, `synonym: <term>` (adds the synonym to the canonical and closes), and `stand by` (closes without changes). Anything else is left open for the next pass.

All three subcommands are one-shot and idempotent. The `serve` subcommand (hosted, GitHub-mode) ships with v0.2; see `docs/PLAN.md`.

## Architecture (compressed)

A single Go binary. The codebase is structured around two seams: cognition (the LLM, planned) and storage (filesystem today, Firestore in v0.2). Two modes will share the same binary:

- **Local CLI** (today): run from your terminal against a repo on disk. The binary reads the working tree, looks for the ubiquitous language file, and writes everything (glossary, log, conversation) into `.ocp/` as plain files. No network. The cognition seam exists but is currently filled by a keyword parser; Vertex / Gemini wiring lands in v0.2.
- **Remote** (v0.2): deployed as a service against a GitHub repo. The agent uses GitHub for the conversation: issues for observations, comments for replies, PRs for glossary edits. Cross-repo state (memory, schedule cursors) lives in Firestore.

See `docs/ARCHITECTURE.md` for detail and `docs/PLAN.md` for the v0.1 to v0.5 roadmap.

## Background: Iain M. Banks

The project name is itself a Banks reference. In *Excession* (1996), an Outside Context Problem is the sort of thing most civilizations encounter just once, in the way a sentence encounters a full stop: an island tribe seeing a much larger ship sail into the bay one morning, the categories they live by suddenly unable to file the new thing. A codebase encounters small OCPs constantly. A new word for an existing concept slips in. A canonical term gets quietly overloaded. The team's shared context erodes one diff at a time. The pun on the engineer's Open-Closed Principle (open for extension, closed for modification) is intentional. The glossary is closed for casual modification, open for extension by consent. Both meanings hold.

The voice and naming convention come from Banks's [Culture novels](https://en.wikipedia.org/wiki/Culture_series) more broadly. In the Culture, the AIs that run civilization-scale infrastructure name themselves with full sentences, speak with idiosyncratic taste, and do their work for the substance of the work. OCP is not a Culture Mind in any literal sense, it is a small agent with narrow scope. The Banks reference is background flavor and discipline: speak rarely, sign your work, prefer one good observation over ten noisy ones.

Each deployed instance picks a Banks-style ship-name from a curated pack on first run. The dogfood instance is `Drone Honor Thy Error As A Hidden Intention`.

## Contributing

See `CONTRIBUTING.md` for the workflow. `AGENTS.md` is the shared contract for humans and AI assistants alike; `CLAUDE.md` adds Claude-specific guidance on top. Security reports go to me@chrisellis.dev (`SECURITY.md`).

## License

Apache 2.0.

## Acknowledgements

- Matt Pocock, whose [skills](https://github.com/mattpocock/skills) and talk *Software Fundamentals Matter More Than Ever* are the project's thesis made into tools.
- Mario Zechner, whose [pi-mono](https://github.com/badlogic/pi-mono) is the harness pattern this Go port is built against.
- John Ousterhout (*A Philosophy of Software Design*), Eric Evans (*Domain-Driven Design*), Kent Beck, Frederick Brooks, Brian Eno, Iain M. Banks. The latent space.
