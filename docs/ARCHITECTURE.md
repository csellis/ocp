# Architecture

This document is the technical reference. For the worldview, read `docs/THESIS.md`. For the roadmap, read `docs/PLAN.md`.

## One binary, two modes

OCP ships as a single Go binary. The same binary runs in two modes:

- **Mode A: Local CLI.** Interactively invoked from the developer's machine. State lives in `.ocp/` inside the watched repo. No network beyond Vertex calls. This is the v0.1 surface and the default way to use OCP.
- **Mode B: Cloud Run service.** Automatically invoked by GitHub webhooks (via Pub/Sub) and by Cloud Scheduler. State lives in Firestore. Speaks back to the watched repo via the GitHub API. This is the v0.2 surface, layered on top of the v0.1 work.

The two modes are peers in shape, not phases of a maturity curve. The work the agent does in either mode is the same: read the repo, look for drift, return an observation. What differs is who pulls the trigger and where the conversation lives.

The mode is selected by the entry-point command (`ocp scan`, `ocp drift`, `ocp respond`, `ocp serve`) and the binary's storage and trigger interfaces are wired up at boot.

This structure exists to enforce a deep-module discipline at the project level. Cognition is the interior. Triggers and storage are the interface seams. Anything in cognition must work identically across both modes.

## Layered diagram

```
            ┌─────────────────────────────────────┐
            │            cmd/ocp                  │
            │  (scan | watch | serve | respond)   │
            └─────────────────────────────────────┘
                          │
                          ▼
            ┌─────────────────────────────────────┐
            │         internal/triggers           │  Interface
            │  cli   |   webhook   |   scheduler  │  3 impls
            └─────────────────────────────────────┘
                          │
                          ▼
            ┌─────────────────────────────────────┐
            │          internal/agent             │
            │   pi-style stateful Agent + tools   │
            │      (turn loop, hooks, events)     │
            └─────────────────────────────────────┘
                  │                    │
                  ▼                    ▼
            ┌──────────┐         ┌──────────────┐
            │cognition │         │    tools     │
            │ vertex   │         │ parse_diff,  │
            │ (Gemini) │         │ glossary,    │
            └──────────┘         │ find_uses,   │
                                 │ gh_issues,   │
                                 │ gh_comments  │
                                 └──────────────┘
                          │
                          ▼
            ┌─────────────────────────────────────┐
            │         internal/storage            │  Interface
            │       filesystem  |  firestore      │  2 impls
            └─────────────────────────────────────┘
```

## Pi-mono primitives, ported to Go

We port the subset of `pi-mono` primitives that OCP needs for v0.1. The rest wait.

### Ported for v0.1

- **Stateful Agent**. A struct holding system prompt, model handle, tool set, and message slice. Methods: `Prompt(ctx, msg) error`, `Continue(ctx) error`, `WaitForIdle(ctx) error`.
- **Turn loop**. One LLM call plus tool executions per turn. Loops while the assistant message contains tool calls. Exits when the assistant produces text without tool calls, or when any tool returns `Terminate: true` and all sibling tools agree.
- **Parallel tool execution**. Tools execute concurrently within a batch via `errgroup`. Results are emitted in source order.
- **Tool definition**. A `Tool` interface with `Name() string`, `Description() string`, `Schema() json.RawMessage`, `Execute(ctx, args) (Result, error)`.
- **Event channel**. Agents emit events on a `chan AgentEvent`. Subscribers consume via range. Idiomatic Go replacement for pi's TypeScript subscriber pattern.
- **`transformContext` hook**. Function called before each LLM call to inject the current glossary, prune old messages, or apply other shaping.
- **`convertToLlm` hook**. Filters and converts custom message types (drift events, oblique cards) to LLM-native `user`/`assistant`/`toolResult`.
- **`afterToolCall` hook**. Audits tool results before they ship. Used to validate issue bodies before they are filed.
- **Custom message types**. `DriftEvent`, `ObliqueCard`, `Observation` extend the base `AgentMessage`. They flow through agent state but are stripped before LLM calls.
- **`Terminate: true`**. Tools can signal early loop exit. Used by `file_issue` to end the agent run cleanly after a single observation has shipped.

### Deferred to v0.2 or later

- Steering / follow-up queues (only matter for long-lived agents; v0.1 agents are short-lived per issue).
- `beforeToolCall` hook (only `afterToolCall` is needed for v0.1).
- Sequential tool mode.
- Multi-provider beyond Vertex.
- Streaming UI events (Cloud Run has no UI; local mode has terminal output only).
- `continue()` retries.

## Cognition

Cognition is an interface:

```go
type Cognition interface {
    Stream(ctx context.Context, prompt Prompt) (StreamReader, error)
}
```

`Prompt` carries the system prompt, the message slice, the tool set, and the thinking budget. `StreamReader` is a typed channel of cognition events (text deltas, tool calls, tool result acknowledgments, usage stats, stop).

For v0.1 we have one implementation: `internal/cognition/vertex`. It uses the Vertex AI Go SDK to stream against Gemini. The model identifier is configurable per agent run.

The default model for v0.1 is **Gemini 2.5 Flash**. Justification: the cheap-stage cascade exists precisely so the LLM call is rare; when it does fire, the work is judging a single drift candidate against a small glossary, not multi-step reasoning. Flash is the right capability/price point for that shape of work. Pro is reserved for tasks the eval harness shows Flash cannot handle, which we expect to be none for synonymy detection. The model identifier is configurable so a future operator can override.

Provider abstraction exists from day one even though we ship one provider. This is not premature abstraction. It is a seam that keeps cognition swappable, lets the eval harness mock cognition with deterministic fixtures, and protects us if Vertex pricing or availability changes.

## Tools

Tools are the units of action. Every tool is independently testable. v0.1 tools:

- `parse_diff(ctx, since RevisionHash) []FileChange`. Reads git diff between HEAD and `since`. Returns structured changes (path, hunks, terms touched).
- `read_glossary(ctx) Glossary`. Loads `.ocp/glossary.md` into a structured `Glossary` value.
- `update_glossary(ctx, change GlossaryChange) error`. Applies a change to `.ocp/glossary.md`. Atomic write.
- `find_term_uses(ctx, term string) []Usage`. AST-walk plus regex fallback to find every site where a term appears. Returns file path, line, surrounding context.
- `surface_observation(ctx, body ObservationBody) ObservationRef`. Surfaces an observation. In Mode A, writes a markdown file to `.ocp/conversation/`. In Mode B, files a GitHub Issue. Returns a stable reference.
- `read_replies(ctx, ref ObservationRef) []Reply`. Fetches all replies on an open observation. Conversation file in Mode A, issue comments in Mode B.
- `add_reply(ctx, ref ObservationRef, body string) error`. Adds a reply.
- `close_observation(ctx, ref ObservationRef, body string) error`. Adds a closing reply and closes the observation.

Each tool has a typed input struct, a typed output struct, and a `Schema()` method that returns the JSON schema Vertex/Gemini consumes for tool calling.

## Triggers

A `Trigger` invokes the agent with the right context for the situation. Three implementations:

- `cli`: invoked from `cmd/ocp`. Reads the current repo, runs one drift or one respond cycle, exits. The v0.1 surface.
- `webhook`: HTTP handler that receives a GitHub webhook (via Pub/Sub fan-out in production), parses the event, and invokes a drift or respond cycle. The v0.2 surface.
- `scheduler`: HTTP handler triggered by Cloud Scheduler on a cadence the team picks. Invokes a scheduled drift run. The v0.2 surface.

Each trigger constructs the agent run with the appropriate initial state and calls `agent.Prompt(ctx, ...)`.

## Storage

A `Storage` interface holds the project's persistent state:

```go
type Storage interface {
    LoadGlossary(ctx context.Context, repo RepoID) (Glossary, error)
    SaveGlossary(ctx context.Context, repo RepoID, g Glossary) error
    AppendLog(ctx context.Context, repo RepoID, entry LogEntry) error
    LoadOpenIssues(ctx context.Context, repo RepoID) ([]IssueRef, error)
    RecordIssueState(ctx context.Context, repo RepoID, issue IssueState) error
}
```

Two implementations:

- `filesystem`: writes to `.ocp/glossary.md`, `.ocp/log.md`, and `.ocp/conversation/` (one file per open observation) inside the repo. Used in Mode A.
- `firestore`: documents per repo, keyed by `owner/name`. Glossary is a single doc; log is a subcollection; observation state is a subcollection. Used in Mode B.

The `filesystem` impl is the v0.1 ship-blocker. The `firestore` impl ships in v0.2.

## The fleet-of-short-lived-agents pattern

There is no single long-running OCP agent process. Instead, **each open observation or each new drift event spawns a short-lived agent run** with the same configured tools and shared glossary state.

Two run shapes:

- **Drift run**: triggered by a `cli` invocation in Mode A, by a webhook or scheduler tick in Mode B. Inputs: glossary, recent diff. Tools: `parse_diff`, `find_term_uses`, `surface_observation`. Expected output: zero or one surfaced observation. Run terminates after `surface_observation` returns or after the agent decides nothing needs surfacing.
- **Conversation run**: triggered by a `cli` invocation in Mode A reading replies in `.ocp/conversation/`, by a webhook on an issue comment in Mode B. Inputs: observation body, all replies, current glossary. Tools: `update_glossary`, `add_reply`, `close_observation`. Expected output: zero or one of (glossary update, reply, close). Run terminates after the chosen action.

Both run shapes use the same `Agent` struct, the same cognition, the same hooks. The tool set is the same shape; behind each tool is a Mode-A or Mode-B implementation (filesystem or GitHub API).

This pattern eliminates the multi-issue cross-talk problem and makes each run independently evaluable.

## Cost discipline: two-stage cascade

The naive design (every tick reads the whole repo, calls Vertex, judges) is unaffordable. The disciplined design is a two-stage cascade:

1. **Cheap stage**, no LLM. Pure Go. Parse the diff since the last tick. Tokenize identifiers and comments. Compare against the glossary. Surface candidate-drift events using regex, AST walk, and Levenshtein distance. Threshold to control noise. Zero tokens.
2. **Expensive stage**, only for candidates that pass the cheap stage. Hydrate an Agent with the candidate, the relevant code excerpts, and the glossary. Ask the model to judge: is this real drift, what flavor, and what is the cleanest single observation to file. Tokens spent only when justified.

The cheap stage exists in `internal/scout`. It is a normal Go package with table-driven tests and zero LLM dependencies. The Agent is invoked only on the output of `scout`.

This cascade is itself a deep-module example. Simple interface (`scout.Detect(diff, glossary) []Candidate`), complex interior (parsers, distance metrics, AST walkers).

## Voice and naming

OCP-authored content has a fixed voice. See `CLAUDE.md` for the template. The voice is calm, observational, dryly precise. The signature line is the deployed instance's ship-name.

Each deployment picks a ship-name from `internal/names/pack.go` on first run. The pack is a curated list of Banks-style names (`Drone Honor Thy Error As A Hidden Intention`, `GCU Conscientious Objector`, `Mind Quietly Tending The Names`, `Drone Steady Drift`). The name is recorded in `.ocp/config.toml` and persists.

## Eval harness

OCP is evaluable. `eval/` contains:

- `corpus/`: hand-labeled drift examples drawn from real OSS repositories. Each example is a directory with `before/`, `after/`, `glossary.md`, and `expected.json` (the labeled drift events that should be detected).
- `eval.go`: runs the cheap stage and the full agent against each example, computes precision and recall, prints a markdown report.

The corpus is part of the open-source project. Contributions are welcome and labeled.

## What this gets us

- A real Go service on GCP using Vertex AI, deployed as Cloud Run, exercising Pub/Sub, Cloud Scheduler, Firestore, and the GitHub API.
- A pi-style agent kit ported to Go, demonstrating turn-loop primitives, tool execution, and hook lifecycle.
- A two-stage cascade demonstrating cost discipline.
- An eval harness demonstrating the project takes correctness seriously.
- A repo that runs OCP on itself, demonstrating the project takes its own thesis seriously.

## What this does not get us

- A general agent framework. OCP is single-purpose. Pi-mono is the framework, in TypeScript. We port a subset.
- A multi-tenant SaaS. v0.2 is the first multi-tenant cut. v0.5 might be a hosted GitHub App. Not the priority.
- Code generation. OCP does not write code. Other agents do. OCP watches the design.
