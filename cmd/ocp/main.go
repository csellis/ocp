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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/csellis/ocp/internal/scout"
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
			Synonyms:   []string{"ubiquitous language", "vocabulary"},
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

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect glossary drift in the working tree and persist candidates as observations.",
	Long: `Drift walks the current working directory looking for words your
glossary lists as synonyms of canonical terms. Each unique
(synonym, file) pair is filed as one observation under
.ocp/conversation/.

Behavior:
  - Loads .ocp/glossary.md. If missing, prints a hint to run 'ocp
    scan' first and exits 0.
  - Walks .go, .md, and .toml files. Skips .git/, .ocp/, hidden
    directories, node_modules, vendor, bin, dist.
  - Groups hits by (synonym, file) pair. For each pair NOT already
    represented by an observation file (open or closed), writes a
    new observation at:
        .ocp/conversation/<NNNN>-<synonym>-<file>.md
  - Prints a summary: how many candidates were found, how many were
    new (filed), how many already had an observation.

Idempotent: re-running with no code changes files no new observations.

Exit codes:
  0  success
  1  unrecoverable error (unreadable file, write failure, malformed glossary)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cwd: %w", err)
		}
		return runDrift(cmd.Context(), cmd.OutOrStdout(), cwd, time.Now().UTC())
	},
}

func runDrift(ctx context.Context, out io.Writer, root string, now time.Time) error {
	fs := storage.New(root)
	g, err := fs.LoadGlossary(ctx, storage.RepoID(""))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			fmt.Fprintln(out, "no glossary at .ocp/glossary.md; run `ocp scan` to seed one")
			return nil
		}
		return fmt.Errorf("load glossary: %w", err)
	}

	hits, err := scout.Detect(ctx, root, g)
	if err != nil {
		return fmt.Errorf("scout: %w", err)
	}

	if len(hits) == 0 {
		fmt.Fprintln(out, "no drift detected")
		return nil
	}

	candidates := groupHits(hits)

	existing, err := fs.AllIssueRefs(ctx, storage.RepoID(""))
	if err != nil {
		return fmt.Errorf("list existing observations: %w", err)
	}
	existingSlugs := map[string]bool{}
	maxNumber := 0
	for _, ref := range existing {
		existingSlugs[slugFromPath(ref.Path)] = true
		if ref.Number > maxNumber {
			maxNumber = ref.Number
		}
	}

	newCount := 0
	skipCount := 0
	nextNumber := maxNumber + 1
	for _, c := range candidates {
		slug := slugify(c.Synonym) + "-" + slugify(c.Canonical)
		if existingSlugs[slug] {
			skipCount++
			continue
		}
		state := storage.IssueState{
			Ref: storage.IssueRef{
				Number: nextNumber,
				Path:   fmt.Sprintf("%04d-%s.md", nextNumber, slug),
			},
			Status:  storage.IssueOpen,
			Updated: now,
			Body:    candidateBody(c),
		}
		if err := fs.RecordIssueState(ctx, storage.RepoID(""), state); err != nil {
			return fmt.Errorf("write observation %s: %w", state.Ref.Path, err)
		}
		newCount++
		nextNumber++
	}

	fmt.Fprintf(out, "%d %s: %d new (filed), %d existing\n",
		len(candidates), pluralize("candidate", len(candidates)), newCount, skipCount)
	return nil
}

// candidate is one observation-worth of evidence: every place a single
// synonym drifts from a single canonical, across the whole tree.
type candidate struct {
	Synonym   string
	Canonical string
	Files     []fileCitation // first-seen order
}

// fileCitation is the per-file evidence inside a candidate: which lines
// in this file contain the synonym.
type fileCitation struct {
	File  string
	Lines []int // sorted, deduped
}

func groupHits(hits []scout.Hit) []candidate {
	// Key on (synonym lowercased, canonical). Synonyms are case-insensitive
	// in scout, so "Vocabulary" and "vocabulary" fold to one candidate.
	// Multiple matches on the same line dedupe to one citation.
	type key struct{ syn, canonical string }
	type bag struct {
		fileOrder []string
		lines     map[string]map[int]bool
	}
	bags := map[key]*bag{}
	out := map[key]candidate{}
	var order []key

	for _, h := range hits {
		synLower := strings.ToLower(h.Synonym)
		k := key{syn: synLower, canonical: h.Canonical}
		b, ok := bags[k]
		if !ok {
			b = &bag{lines: map[string]map[int]bool{}}
			bags[k] = b
			out[k] = candidate{Synonym: synLower, Canonical: h.Canonical}
			order = append(order, k)
		}
		if b.lines[h.File] == nil {
			b.lines[h.File] = map[int]bool{}
			b.fileOrder = append(b.fileOrder, h.File)
		}
		b.lines[h.File][h.Line] = true
	}

	result := make([]candidate, 0, len(order))
	for _, k := range order {
		c := out[k]
		b := bags[k]
		for _, file := range b.fileOrder {
			lines := make([]int, 0, len(b.lines[file]))
			for ln := range b.lines[file] {
				lines = append(lines, ln)
			}
			sort.Ints(lines)
			c.Files = append(c.Files, fileCitation{File: file, Lines: lines})
		}
		result = append(result, c)
	}
	return result
}

func candidateBody(c candidate) string {
	var b strings.Builder
	totalLines := 0
	for _, f := range c.Files {
		totalLines += len(f.Lines)
	}
	fmt.Fprintf(&b, "Synonym `%s` appeared in %d %s (%d %s). The glossary canonicalizes this concept as `%s`.\n\n",
		c.Synonym, len(c.Files), pluralize("file", len(c.Files)),
		totalLines, pluralize("occurrence", totalLines), c.Canonical)
	b.WriteString("Citations:\n")
	for _, f := range c.Files {
		fmt.Fprintf(&b, "- %s:", f.File)
		for i, ln := range f.Lines {
			if i == 0 {
				fmt.Fprintf(&b, " %d", ln)
			} else {
				fmt.Fprintf(&b, ", %d", ln)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// slugify returns a filesystem-safe lowercase slug. Non-alphanumeric
// runs collapse to single dashes; leading and trailing dashes are
// trimmed. Used for observation filenames.
func slugify(s string) string {
	var b strings.Builder
	lastDash := true // leading dash suppression
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

var slugStripRe = regexp.MustCompile(`^\d+-(.+)$`)

// slugFromPath extracts the slug part of an observation filename:
// "0001-vocabulary-docs-thesis-md.md" -> "vocabulary-docs-thesis-md".
// Returns the input (minus extension) if the NNNN- prefix is absent.
func slugFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	if m := slugStripRe.FindStringSubmatch(base); len(m) == 2 {
		return m[1]
	}
	return base
}

func pluralize(s string, n int) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(driftCmd)
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ocp: %v\n", err)
		os.Exit(1)
	}
}
