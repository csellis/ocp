# OCP TODO

Maintainer scratchpad. Not a roadmap. For the roadmap, see `docs/PLAN.md`.

## Tooling and harness

- [ ] Run a Go-flavored pass of the `/harness-engineering` skill against this repo.
      Audit project constraints, review the agent setup, design test harnesses,
      and harden `CLAUDE.md` plus linter rules for agentic coding. Tailor the
      output to Go idioms (table-driven tests, `t.Helper`, `t.Cleanup`, build
      tags, `errgroup`, `context.Context` propagation) rather than the
      TS/SvelteKit defaults the skill assumes.

- [ ] Add `.github/workflows/` for release builds. Releases are NOT CI/CD;
      commits to `main` flow freely. A release is cut manually (tag push or
      `workflow_dispatch`) and the workflow produces signed binaries for
      darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and attaches them
      to the GitHub Release. Decide between `goreleaser` (fast, conventional)
      and a hand-rolled matrix workflow (one fewer dependency, more code).

- [ ] Build a release-drift backpressure harness. Goal: keep moving fast on
      `main`, but raise the urgency to cut a release as the lag from the last
      tag grows. The voice escalates the way OCP itself escalates: subtle at
      small lag, urgent at large lag, never spammy. This is OCP's own thesis
      applied to release cadence. Sketch:
      - measure: commits since last tag, days since last tag, lines of diff
      - render: a status check, a sticky issue, or a `make release-status`
        line that the dev sees on every `make all`
      - escalate: tier 1 (informational) -> tier 2 (recommend cutting) ->
        tier 3 (block non-trivial PRs until release or explicit override)
      - the override matters: backpressure that cannot be bypassed becomes
        ceremony. Bypass must be one flag and must be logged.
