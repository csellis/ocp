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

### NEXT — bubbletea migration for the interactive surface

- [ ] **DO THIS FIRST.** Migrate the home menu and `ocp respond` TUI
      from line-buffered `bufio.Reader` + `fmt.Fprintln` to a real TUI
      library: `github.com/charmbracelet/bubbletea`.

      Why: line-buffered terminal mode means bare ESC is invisible to
      the program (the byte sits in the kernel buffer, no read fires
      until Enter). Workarounds (empty-enter cancel, raw-mode-only-at-
      menu) all hit the same wall — sub-prompts where you type a reason
      or term still don't get ESC. Real ESC requires raw mode; raw mode
      breaks line editing unless you re-implement it; bubbletea has
      already done that work.

      Wins beyond ESC:
      - Real navigation (arrow keys, ESC, ctrl-c) at every prompt
      - Redraw on every keystroke; legend / status updates in place
      - Scroll regions for [d]etails (no more bottom-of-screen vomit)
      - Cleaner separation of model / view / update (Elm-architecture)
      - Foundation for v0.2 polish (multi-pane, live status during
        long drift runs, etc.)

      Migration scope:
      - Home (status block + action menu) becomes a bubbletea Model
      - Respond (legend + per-observation walk + sub-prompts) is a
        second Model that runs as a child program
      - File-Reply path (`--from-file`) stays untouched; bubbletea
        only owns the interactive front-end
      - Tests: bubbletea has a `tea.NewProgram(...).Run()` test mode
        with scripted input. Existing TUI unit tests rewrite to drive
        the Model directly (Init/Update with simulated tea.Msg values),
        which is cleaner than current bufio scripting.
      - Dep: bubbletea + lipgloss (styling) + bubbles (input field).
        Reasonable for a TUI app; biggest dep we have aside from cobra.

      Estimate: ~5 sprint points. Half a day to spike the home Model;
      half a day to migrate respond; rest is polish + test rewrite.

      Reference: https://github.com/charmbracelet/bubbletea

- [ ] (Old, superseded by NEXT above): TUI for `ocp respond`. Today's
      bufio-based TUI shipped in slice 9 but ESC doesn't work in
      line-buffered mode. Bubbletea migration replaces it.

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
