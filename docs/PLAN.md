# Plan

This is the operational roadmap. For the technical architecture, read `docs/ARCHITECTURE.md`. For the worldview, read `docs/THESIS.md`.

## Versions

- **v0.1**: local CLI, dogfood instance, file-system storage. Interactive invocation only. Ship gate is "OCP scans its own repo, surfaces an observation about its own glossary, and a human-readable response causes OCP to update the glossary and close the observation."
- **v0.2**: Cloud Run deployment, Firestore storage, GitHub webhook + Pub/Sub trigger, Cloud Scheduler. The first automatic-invocation surface. Ship gate is "OCP runs as a hosted service and demonstrates the same workflow against a second OSS repo."
- **v0.3**: ambiguity and vagueness modes (v0.1 ships synonymy detection); the headlights detector (Pocock failure mode #4); module-depth-erosion mode.
- **v0.5**: design-concept divergence (Pocock failure mode #3); specs-to-code drift detector; possibly a hosted GitHub App.

Versions ship when the gate passes, not on a calendar.

## v0.1 ship gates

A v0.1 release is gated on these passing in order:

1. `ocp scan` runs locally. If the repo has no ubiquitous language file, it generates `.ocp/glossary.md` from the codebase. If one exists, it reads it. Exits cleanly.
2. `ocp drift` runs locally against the current diff, detects synonymy drift, writes one observation per drift event into `.ocp/conversation/` with a citation to the diff. One-shot, foreground, exits.
3. `ocp respond` reads replies in `.ocp/conversation/` (or comments on OCP-filed issues when remote), takes one of: update glossary, add clarifying note, close. Closes are accompanied by a one-line reason. One-shot, foreground, exits.
4. The repo runs OCP on itself. `.ocp/log.md` has at least one self-observation. The glossary in `.ocp/glossary.md` is non-empty.
5. The eval harness runs against `eval/corpus/` (minimum 5 hand-labeled examples) and reports precision and recall on the synonymy task.
6. README opens with the Pocock-thesis paragraph plus Beck quote. AGENTS.md is committed. License is Apache 2.0.

When all six pass, tag v0.1.0 and write the launch post.

## File structure (v0.1)

```
ocp/
├── README.md
├── AGENTS.md
├── CLAUDE.md
├── CLAUDE.local.md          (gitignored)
├── LICENSE                  (Apache 2.0)
├── NOTICE
├── .gitignore
├── .ocp/
│   ├── config.toml          (instance ship-name, opt-in flags)
│   ├── glossary.md          (OCP's own glossary)
│   └── log.md               (OCP's observations on itself)
├── go.mod
├── go.sum
├── cmd/
│   └── ocp/
│       └── main.go          (CLI entry; subcommands: scan drift respond serve)
├── internal/
│   ├── agent/               (pi-style primitives)
│   │   ├── agent.go         (Agent struct, turn loop)
│   │   ├── agent_test.go
│   │   ├── tool.go          (Tool interface)
│   │   ├── message.go       (AgentMessage + custom types)
│   │   ├── context.go       (transformContext, convertToLlm)
│   │   └── events.go        (event channel types)
│   ├── cognition/
│   │   ├── cognition.go     (interface)
│   │   └── vertex/
│   │       ├── vertex.go    (Vertex / Gemini impl)
│   │       └── vertex_test.go
│   ├── scout/
│   │   ├── scout.go         (cheap detector, no LLM)
│   │   ├── synonymy.go
│   │   ├── ambiguity.go     (stub for v0.3)
│   │   ├── vagueness.go     (stub for v0.3)
│   │   └── scout_test.go
│   ├── tools/
│   │   ├── parse_diff.go
│   │   ├── glossary.go
│   │   ├── find_term_uses.go
│   │   ├── github_issues.go
│   │   └── *_test.go
│   ├── triggers/
│   │   ├── trigger.go       (interface)
│   │   ├── cli.go
│   │   ├── webhook.go       (stub for v0.2)
│   │   └── scheduler.go     (stub for v0.2)
│   ├── storage/
│   │   ├── storage.go       (interface)
│   │   ├── filesystem.go    (v0.1)
│   │   ├── firestore.go     (stub for v0.2)
│   │   └── *_test.go
│   ├── names/
│   │   ├── pack.go          (Banks-style ship names)
│   │   └── pack_test.go
│   └── voice/
│       ├── observation.go   (formats issue bodies in OCP voice)
│       ├── oblique.go       (Oblique Strategy card pack)
│       └── *_test.go
├── docs/
│   ├── THESIS.md
│   ├── ARCHITECTURE.md
│   ├── PLAN.md              (this file)
│   └── images/
└── eval/
    ├── corpus/              (hand-labeled examples)
    │   ├── 001-synonymy-eval-vs-assessment/
    │   ├── 002-synonymy-fetch-vs-load/
    │   └── ...
    ├── eval.go              (eval runner)
    └── README.md
```

## Order of operations

The order below is the build order. Each step is a small PR. None ship code without tests.

1. `go.mod` initialized. `golangci-lint` config in place. `make` or `taskfile` runs lint + test + build.
2. `internal/storage/filesystem.go` with table-driven tests. The simplest piece. Pure file I/O.
3. `internal/agent/message.go` and `internal/agent/events.go`. Just types. No behavior.
4. `internal/cognition/cognition.go` interface. `internal/cognition/vertex` with a fake / replay-from-fixtures mode for testing. The real Vertex client wired but gated on credentials.
5. `internal/agent/agent.go` and `agent_test.go`. The turn loop. Test against the fake cognition.
6. `internal/agent/tool.go` plus the simplest two real tools: `read_glossary` and `update_glossary`. Round-trip test.
7. `internal/scout/synonymy.go`. The cheap detector. Pure Go. Table-driven tests.
8. `internal/tools/parse_diff.go` and `find_term_uses.go`. Real git operations.
9. `internal/voice/observation.go` and `internal/voice/oblique.go`. Issue-body formatting.
10. `internal/tools/github_issues.go`. Real GitHub API calls; tests use fixtures.
11. `cmd/ocp/main.go` `scan` subcommand. End-to-end: read or generate glossary, exit.
12. `cmd/ocp/main.go` `drift` subcommand. One-shot: scout, agent run, write observation to `.ocp/conversation/`. Exits.
13. `cmd/ocp/main.go` `respond` subcommand. One-shot: read replies in `.ocp/conversation/`, conversation agent run, write resolution. Exits.
14. `eval/eval.go` and corpus seeded with at least 5 examples.
15. Run OCP on itself. Iterate until `.ocp/log.md` has a defensible first entry.
16. README polish, launch post, tag v0.1.0.

## v0.2 build order (after v0.1 ships)

The v0.2 question is which automatic-invocation surface to ship first. Webhook on push is the most responsive; scheduler on cadence is the most predictable; GitHub Action on PR is the most discoverable. Pick one, ship it, learn.

1. `internal/storage/firestore.go`. Same interface, Firestore documents.
2. `internal/triggers/webhook.go`. HTTP handler for GitHub webhooks. Pub/Sub fan-out.
3. `internal/triggers/scheduler.go`. HTTP handler for Cloud Scheduler.
4. `Dockerfile` and Cloud Run deployment manifests.
5. Terraform module for the GCP resources.
6. End-to-end smoke test against a second OSS repo (with permission).

Future consideration: optional anonymized session publication (see pi-mono precedent). Out of scope until there are enough real runs to know whether the corpus would be useful.

## What is explicitly out of scope for v0.1

- Multi-provider cognition. Vertex only.
- Steering / follow-up queues.
- Ambiguity and vagueness drift modes (synonymy only for v0.1).
- Module-depth-erosion mode.
- A web UI.
- A hosted GitHub App.
- A SaaS billing layer.

## Risks and how we manage them

- **Risk: cost overruns from naive LLM calls.** Managed by the cheap-detector cascade. Every Vertex call is gated on a `scout` candidate.
- **Risk: noisy issues.** Managed by threshold tuning and the eval harness. If precision drops below an agreed bar, the threshold tightens.
- **Risk: drift in the project's own glossary while building.** Managed by AGENTS.md discipline (every new term goes in `.ocp/glossary.md`) and by running OCP on itself from week 2.
- **Risk: scope creep into module-depth analysis.** Managed by versioning. v0.1 is synonymy only. Module depth waits.
- **Risk: writing a Go agent framework instead of OCP.** Managed by porting only the pi-mono primitives we need. The framework is pi-mono. We are not rewriting it.

## What success looks like at v0.1

A second engineer (anyone) can clone this repo, install the binary, point it at a small Go project, and within ten minutes have:

- A glossary at `.ocp/glossary.md`.
- An issue filed on the watched repo (or a clean exit if no drift was detected).
- An eval report from the local corpus showing OCP's precision and recall on synonymy.

When that demo runs cleanly, v0.1 is done.
