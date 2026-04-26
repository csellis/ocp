# OCP TODO

Maintainer scratchpad. Not a roadmap. For the roadmap, see `docs/PLAN.md`.

## Tooling and harness

- [ ] Run a Go-flavored pass of the `/harness-engineering` skill against this repo.
      Audit project constraints, review the agent setup, design test harnesses,
      and harden `CLAUDE.md` plus linter rules for agentic coding. Tailor the
      output to Go idioms (table-driven tests, `t.Helper`, `t.Cleanup`, build
      tags, `errgroup`, `context.Context` propagation) rather than the
      TS/SvelteKit defaults the skill assumes.
