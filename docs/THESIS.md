# Thesis

## Bad code is the most expensive it has ever been

Matt Pocock's argument, compressed: AI in a good codebase is excellent. AI in a bad codebase compounds entropy. The "specs to code" movement, which says you can ignore the code and let it manage itself, is vibe-coding by another name. Each compile of the spec produces worse code than the last. The specs-to-code idea is not just wrong, it is dangerous, because it asks you to divest from the design of the system at exactly the moment when the cost of bad design is highest.

> If you have a codebase that's hard to change, you're not able to take all of the bounty that AI can offer, because AI in a good codebase actually does really, really well. This means good codebases matter more than ever, which means software fundamentals matter more than ever.
>
> — Matt Pocock, *Software Fundamentals Matter More Than Ever*

The conclusion: software fundamentals matter more in the agentic age, not less.

## The fundamentals are not new

- **Ousterhout**: complexity is anything in the structure that makes the system hard to understand and modify. Good codebases are easy to change. Deep modules (lots of behavior behind a small interface) are the antidote to complexity.
- **Evans**: a domain has a *ubiquitous language*. The team's vocabulary in code, docs, and conversation should match. When it does not, you have drift, and drift is the same shape as software entropy.
- **Brooks**: when more than one person designs together, an ephemeral *design concept* floats between them. It is not in any artifact. The shared understanding is the asset; the artifacts are downstream.
- **Beck**: invest in the design of the system every day.
- **Pragmatic Programmer**: the rate of feedback is the speed limit. Outrunning your headlights produces worse code, faster.

These ideas survived because they are right. AI does not change them. AI raises the cost of ignoring them.

## Pocock's tools are not enough

Pocock built three skills that operationalize parts of this:

- `grill-me`: forces a shared design concept before any code is written.
- `ubiquitous-language`: extracts the team's domain vocabulary into a glossary.
- `improve-codebase-architecture`: finds shallow modules and proposes deepening refactors.

They are useful. They are also one-shot, human-invoked. You run them, they produce a markdown file, you absorb it, you move on. **The drift starts again the moment you close the tab.**

A team's discipline cannot be a tool you remember to run. It has to be a presence.

## OCP is a missing piece

OCP is one continuous version. It works on a codebase in an ambient posture: low temperature, deliberate, in no hurry. It maintains the glossary as a living artifact, not a one-shot scan. It looks for drift in the three classical flavors (synonymy, ambiguity, vagueness) and speaks only when drift exceeds threshold. When it speaks, the speech-act is an observation: a local file when invoked from the CLI, a GitHub Issue when invoked against a remote repo, because issues are how teams already coordinate.

OCP does not write code. OCP does not open PRs against your business logic. OCP only surfaces observations and updates its own `.ocp/` state. The blast radius is bounded by design.

This is one way to operationalize "invest in the design of the system every day." You can run it yourself when you remember, schedule it once you stop, or wire it to a webhook when you want it on every push. The work is the same. Who pulls the trigger is up to you.

## Why an agent, not a chatbot

A chatbot returns answers to questions. The user supplies the question, the chatbot supplies the response, the user does the work of acting on it. The shape is conversational, the unit is the exchange.

OCP is shaped differently. There are no questions from the user. The agent has a job (look at the codebase for drift in the ubiquitous language) and it returns observations to a workspace. The user reads what was observed, decides whether the observation is correct, and either updates the glossary or pushes back. The unit of value is the moment a human reads what OCP wrote and is glad it spoke.

This shape (read code, judge, surface an observation, exit) is small enough to be honest about what the agent is doing and easy enough to evaluate. It is also amenable to multiple invocation models: a CLI you run yourself, a scheduled job that runs without you, a webhook that fires on push. The shape of the work does not change.

The Banks reference (see `README.md` for context) supplies the voice and the naming convention but does not literally claim that OCP is a Culture Mind. It is a small agent with narrow scope. The Banks framing is discipline: speak rarely, sign your work, prefer one good observation over ten noisy ones.

## Posture is ambient, invocation is interactive

There are two questions in any agent design: what is its posture, and how does it get invoked. They are independent, and conflating them is what makes most "ambient agent" projects either over-built or vapor.

Posture is ambient. The work is low-temperature, deliberate, in no hurry. The agent dwells in the codebase rather than scanning it. Brian Eno's term for that mode is *ambient*: music for airports, generative and quiet, designed to be lived with rather than attended to. It is the right posture for noticing drift, where the signal is small and continuous and the wrong answer is to over-react.

Invocation, in v0.1, is interactive. You run `ocp drift` from the terminal. It does its pass, surfaces what it found, exits. This matters: automatic invocation (cron, webhooks, daemons) is a real surface but it is the harder surface to build well, and the easier surface to over-build before any of the actual work is sound. Build the work first. Build the surfaces later.

Humans cannot perceive drift on cadence. They can perceive drift when they look. The interactive surface is not a step backwards from ambient invocation; it is the honest first cut. Once the agent's observations are good enough to be worth reading, the question of automatic invocation gets easier to answer because there is something real to wire up.

The open question for v0.2 is which automatic surface is right: a webhook on push, a cron on a cadence the team picks, a GitHub Action triggered on PR. The work does not change. The trigger does.

## Why GitHub Issues, not Slack

A Slack message is ephemeral. An issue is durable. A Slack message creates pressure to respond now. An issue creates a queue, a triage workflow, and a record. Issues are how engineering teams already coordinate decisions. Filing an issue puts OCP in the team's existing system rather than asking the team to attend to a new channel.

The conversation is the issue thread. OCP reads its own issue's comments and decides: update the glossary, ask for clarification, stand by, or close. The conversation primitive is built into the platform.

## Why Go on GCP (preference, not necessity)

OCP could be built in TypeScript on Cloudflare Workers, in Python on AWS Lambda, in Rust on Fly.io. The architecture is portable. The cognition layer is provider-shaped, not language-shaped. The two-stage cascade is just code. At a certain point, stack choice boils down to preference and what the maintainer wants to build fluency in.

We chose Go on GCP because:

1. **Stack alignment.** This project is a learning vehicle for Go and GCP for the maintainer. The artifact is real OSS, not a tutorial, but the choice of stack is downstream of what the maintainer wants to learn.
2. **Operational fit for the eventual remote mode.** Cloud Run + Pub/Sub + Cloud Scheduler + Firestore is one mature serverless surface for a low-frequency agent. The same Go binary runs locally with file-system state and remotely with Firestore state, which keeps the deep-module discipline (cognition is the interior; triggers and storage are interface seams).
3. **Cost shape.** Go's standard library and the Vertex SDK make the cheap-detector / expensive-LLM cascade tractable to write. A Go binary's cold-start is fast enough that the eventual remote mode is not financially fatal.

These are reasons for *this* implementation. They are not reasons against any other implementation.

## Why pi-mono primitives

Pi-mono (Mario Zechner) is a primitives kit for stateful agents: rich event streaming, hooks (`beforeToolCall`, `afterToolCall`), steering and follow-up queues, custom message types via declaration merging, multi-provider abstraction.

OCP is a small agent with multiple concurrent threads of attention (drift run, conversation on N open observations, glossary maintenance) even when the invocation is one-shot. Each invocation is short, but the shape of the work is small-agent-with-tools, not single-pass-script. That requires primitives. We port the relevant pi-mono primitives to Go.

## The dog-food principle

OCP runs on its own repo from day one. The first observation in `.ocp/log.md` is OCP noticing OCP. The glossary in `.ocp/glossary.md` includes the project's own canonical vocabulary. If OCP cannot maintain its own glossary, it has no business maintaining anyone else's.

This is the test that costs nothing and teaches everything.

## What OCP is not

- Not a linter. Linters are static, stateless, syntactic. OCP has memory, taste, and runs on cadence.
- Not a code-review bot. OCP does not opine on PR quality.
- Not a chatbot. You do not ask OCP questions.
- Not a productivity tool. OCP exists to maintain the design of the system. Productivity gains are a side effect.
- Not a replacement for the team's discipline. OCP is a backstop, not a delegate.

## What success looks like

A team using OCP, six months in, says: "we still have arguments about names, but we have those arguments deliberately, in the glossary, not by accident, in the code." That is the win.
