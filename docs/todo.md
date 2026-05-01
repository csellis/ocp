# OCP TODO

Maintainer scratchpad. Not a roadmap. For the roadmap, see `docs/PLAN.md`.

## Tooling and harness

- [ ] Run a Go-flavored pass of the `/harness-engineering` skill against this repo.
      Audit project constraints, review the agent setup, design test harnesses,
      and harden `CLAUDE.md` plus linter rules for agentic coding. Tailor the
      output to Go idioms (table-driven tests, `t.Helper`, `t.Cleanup`, build
      tags, `errgroup`, `context.Context` propagation) rather than the
      TS/SvelteKit defaults the skill assumes.

- [x] Add `.github/workflows/` for release builds. Shipped in `cc3b04b`
      (`goreleaser`-driven, gated on tag push or `workflow_dispatch`,
      multi-arch binaries attached to GitHub Release). v0.1.0 published
      via this pipeline.

## v0.1.x cleanup

- [x] Migrated the home menu and `ocp respond` TUI from line-buffered
      `bufio.Reader` + `fmt.Fprintln` to `bubbletea` (with `lipgloss`
      and `bubbles/textinput`). ESC works at the menu and inside every
      sub-prompt. Arrow keys + letter shortcuts at home; real line
      editing in close-reason and synonym prompts. Stylist (color.go)
      deleted; both Models share lipgloss-backed styles. File-Reply
      path (`--from-file`) untouched. Tests rewritten to drive Models
      directly via `Update(tea.KeyMsg)` rather than scripted bufio.

      Deferred polish (separate slices later):
      - Scroll region for `[d]etails` so long bodies don't push the
        prompt off-screen.
      - Live status during long `ocp drift` runs (currently silent
        until done).
      - Multi-pane home (status left, log preview right).

- [x] `ocp drift` crashed when the git index referenced files removed
      from disk (e.g., a `rm` not yet `git add`-ed): scout opened the
      missing path and surfaced ENOENT. `internal/scout/scout.go`
      now treats `fs.ErrNotExist` in `scanList` as a silent skip,
      with a regression test (`TestDetect_TrackedButDeletedSkippedSilently`).

- [ ] Re-running `ocp drift` after scout changes does not refresh
      existing observation files (dedupe-by-slug skips them). The only
      way to regenerate is `rm -rf .ocp/conversation/`. Either add
      `--regenerate` or make dedupe content-aware (recompute body, write
      if changed).

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
