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

## What it does (v0.1)

1. Looks for the team's ubiquitous language file (the glossary) in the repo. If none exists, generates one from the codebase on first run and writes `.ocp/glossary.md`. You edit it. Future runs respect your edits.
2. Watches for drift in three flavors:
   - **Synonymy**: multiple words for one concept
   - **Ambiguity**: one word for multiple concepts
   - **Vagueness**: overloaded or imprecise terms
3. Surfaces each drift event as a single observation. In remote mode this is a GitHub Issue. In local CLI mode it is an entry in `.ocp/conversation/`. Each carries citations from the diff and optionally a curated Oblique Strategy card.
4. Reads replies on its own observations (issue comments in remote mode, edits to the conversation file locally). Updates the glossary, asks for clarification, or stands by the observation, then closes.
5. Runs on its own repo from day one. The first observation in `.ocp/log.md` is OCP noticing OCP.

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

Run from inside the repo you want OCP to watch. Today's local-CLI surface is two subcommands.

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

The run is idempotent: re-running with no code changes files no new observations. The remaining `respond` and `serve` subcommands described in the roadmap (see `docs/PLAN.md`) are not yet wired.

## Architecture (compressed)

A single Go binary. Pi-mono-style stateful agent primitives ported to Go. Vertex AI (Gemini) as cognition. Two modes share the same binary:

- **Local CLI**: run from your terminal against a repo on disk. The agent reads the working tree, looks for the ubiquitous language file, and writes everything (glossary, log, conversation) into `.ocp/` as plain files. No GitHub required, no network beyond the model call.
- **Remote**: deployed as a service against a GitHub repo. The agent uses GitHub for the conversation: issues for observations, comments for replies, PRs for glossary edits. State that does not belong in the repo (cross-repo memory, schedule cursors) lives in Firestore.

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
