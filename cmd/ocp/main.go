// Command ocp is the OCP command-line entry point.
//
// Subcommands ship incrementally per docs/PLAN.md. v0.1-dev today:
//
//	scan    read or seed .ocp/glossary.md and print the current state
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/csellis/ocp/internal/storage"
)

const version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "ocp",
	Short: "An ambient agent for ubiquitous-language drift.",
	Long: `OCP watches a codebase for drift in its ubiquitous language: the
canonical names a team has agreed on for the concepts in their domain.
OCP does not write code. It surfaces single observations and updates
its own .ocp/ state. The blast radius is bounded by design.

For the thesis and architecture, see docs/THESIS.md and docs/ARCHITECTURE.md.
For the build roadmap, see docs/PLAN.md.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Read or seed .ocp/glossary.md and print the current state.",
	Long: `Scan operates on the current working directory.

Behavior:
  - If .ocp/glossary.md does not exist, write the seed glossary
    (the project's own canonical vocabulary) and report
    "wrote new glossary at <path>".
  - If .ocp/glossary.md exists, read it and report the term count.
  - In both cases, print the full glossary markdown to stdout.

Exit codes:
  0  success
  1  unrecoverable error (permissions, unreadable file, malformed glossary)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cwd: %w", err)
		}
		return runScan(cmd.Context(), cmd.OutOrStdout(), cwd)
	},
}

// runScan is the testable core of the scan subcommand. Decoupled from
// cobra so tests pass a temp dir and a buffer instead of relying on
// process state.
func runScan(ctx context.Context, out io.Writer, root string) error {
	fs := storage.New(root)
	g, err := fs.LoadGlossary(ctx, storage.RepoID(""))
	seeded := false
	switch {
	case err == nil:
		// existing glossary; nothing to do
	case errors.Is(err, storage.ErrNotFound):
		g = seedGlossary()
		if err := fs.SaveGlossary(ctx, storage.RepoID(""), g); err != nil {
			return fmt.Errorf("save seed: %w", err)
		}
		seeded = true
	default:
		return fmt.Errorf("load glossary: %w", err)
	}

	if seeded {
		fmt.Fprintf(out, "wrote new glossary at %s (%d terms)\n\n", filepath.Join(root, ".ocp", "glossary.md"), len(g.Terms))
	} else {
		fmt.Fprintf(out, "loaded glossary: %d terms\n\n", len(g.Terms))
	}
	out.Write(g.Markdown())
	return nil
}

// seedGlossary returns the dogfood vocabulary OCP writes on first run
// when the watched repo has no .ocp/glossary.md yet. These are the
// project's own canonical terms, honestly declared.
func seedGlossary() storage.Glossary {
	return storage.Glossary{Terms: []storage.Term{
		{
			Canonical:  "OCP",
			Definition: "Outside Context Problem. Open-Closed Principle. Both meanings hold. The project name and the agent itself.",
		},
		{
			Canonical:  "drift",
			Definition: "Movement of a canonical term toward synonymy, ambiguity, or vagueness. The thing OCP detects.",
		},
		{
			Canonical:  "glossary",
			Definition: "The team's ubiquitous language, held as .ocp/glossary.md. OCP reads it on every run; humans edit it; OCP files observations when usage drifts from canonical.",
		},
		{
			Canonical:  "scout",
			Definition: "The cheap-stage detector. Pure Go, zero LLM calls. Stage one of the two-stage cascade.",
		},
		{
			Canonical:  "observation",
			Definition: "An OCP-authored issue body. The unit of OCP's speech-act. Local file under .ocp/conversation/ in Mode A; GitHub Issue in Mode B.",
		},
		{
			Canonical:  "ship-name",
			Definition: "The deployed instance's Banks-style name. Picked from the names pack on first run, recorded in .ocp/config.toml.",
		},
		{
			Canonical:  "eval",
			Definition: "An evaluation pass against the labeled corpus. The eval harness lives in eval/.",
		},
	}}
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ocp: %v\n", err)
		os.Exit(1)
	}
}
