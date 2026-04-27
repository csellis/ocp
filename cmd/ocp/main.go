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

	"github.com/csellis/ocp/internal/names"
	"github.com/csellis/ocp/internal/scout"
	"github.com/csellis/ocp/internal/storage"
	"github.com/csellis/ocp/internal/voice"
)

// Build-time metadata. Overridden via -ldflags "-X main.version=..." by
// the release pipeline (.goreleaser.yaml). Defaults are what `go install`
// from a working tree produces.
var (
	version = "0.1.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ocp",
	Short: "An ambient agent for ubiquitous-language drift.",
	Long: `OCP watches a codebase for drift in its ubiquitous language: the
canonical names a team has agreed on for the concepts in their domain.
OCP does not write code. It surfaces single observations and updates
its own .ocp/ state. The blast radius is bounded by design.

For the thesis and architecture, see docs/THESIS.md and docs/ARCHITECTURE.md.
For the build roadmap, see docs/PLAN.md.`,
	Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
	SilenceUsage:  true,
	SilenceErrors: true,
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Read or seed .ocp/glossary.md and print the current state.",
	Long: `Scan operates on the current working directory.

Behavior:
  - If .ocp/glossary.md does not exist, write the seed glossary
    (the project's own canonical vocabulary), append a one-line
    self-observation to .ocp/log.md, and report
    "wrote new glossary at <path>".
  - If .ocp/glossary.md exists, read it and report the term count.
    No log entry is written for a no-op load.
  - In both cases, print the full glossary markdown to stdout.

Exit codes:
  0  success
  1  unrecoverable error (permissions, unreadable file, malformed glossary)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cwd: %w", err)
		}
		return runScan(cmd.Context(), cmd.OutOrStdout(), cwd, time.Now().UTC())
	},
}

// runScan is the testable core of the scan subcommand. Decoupled from
// cobra so tests pass a temp dir, a buffer, and a fixed time instead of
// relying on process state.
func runScan(ctx context.Context, out io.Writer, root string, now time.Time) error {
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
		if err := fs.AppendLog(ctx, storage.RepoID(""), storage.LogEntry{
			At:   now,
			Body: fmt.Sprintf("scan seeded glossary (%d terms)", len(g.Terms)),
		}); err != nil {
			return fmt.Errorf("append log: %w", err)
		}
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
	var filed []string
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
			Status:    storage.IssueOpen,
			Updated:   now,
			Body:      candidateBody(c),
			Canonical: c.Canonical,
			Synonym:   c.Synonym,
		}
		if err := fs.RecordIssueState(ctx, storage.RepoID(""), state); err != nil {
			return fmt.Errorf("write observation %s: %w", state.Ref.Path, err)
		}
		newCount++
		nextNumber++
		filed = append(filed, state.Ref.Path)
	}

	fmt.Fprintf(out, "%d %s: %d new (filed), %d existing\n",
		len(candidates), pluralize("candidate", len(candidates)), newCount, skipCount)

	if newCount > 0 {
		var body strings.Builder
		fmt.Fprintf(&body, "drift filed %d %s:", newCount, pluralize("observation", newCount))
		for _, p := range filed {
			fmt.Fprintf(&body, "\n- %s", p)
		}
		if err := fs.AppendLog(ctx, storage.RepoID(""), storage.LogEntry{
			At:   now,
			Body: body.String(),
		}); err != nil {
			return fmt.Errorf("append log: %w", err)
		}
	}
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
	files := make([]voice.FileCitation, len(c.Files))
	for i, f := range c.Files {
		files[i] = voice.FileCitation{File: f.File, Lines: f.Lines}
	}
	return voice.Format(voice.Body{
		Synonym:   c.Synonym,
		Canonical: c.Canonical,
		Files:     files,
		Card:      voice.PickCard(),
		ShipName:  names.Default(),
	})
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

var respondCmd = &cobra.Command{
	Use:   "respond",
	Short: "Read replies on open observations and apply close / synonym actions.",
	Long: `Respond walks every open observation in .ocp/conversation/ and looks
for a "## Reply" section the maintainer has appended to the body.

Recognized reply formats (case-insensitive):

  close: <reason>      Close the observation. The reason is appended to
                       the body and the file moves to conversation/closed/.
  synonym: <term>      Add <term> as a declared synonym of the
                       observation's canonical, then close the
                       observation with a closure note.
  stand by             Leave the observation open. No state change.

Observations without a Reply section (or with an unrecognized reply)
are skipped silently. The summary line reports counts per outcome.

This v0.1 surface is a keyword parser. v0.2 replaces it with the
LLM-driven conversation loop the architecture describes.

Exit codes:
  0  success
  1  unrecoverable error (unreadable observation, glossary write failure)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cwd: %w", err)
		}
		return runRespond(cmd.Context(), cmd.OutOrStdout(), cwd, time.Now().UTC())
	},
}

type replyAction struct {
	kind  replyKind
	value string
}

type replyKind int

const (
	replyNone replyKind = iota
	replyClose
	replySynonym
	replyStandBy
)

// parseReply scans an observation body for a "## Reply" section and
// extracts the recognized intent. Anything outside the recognized
// keyword set returns replyNone (the run skips silently).
func parseReply(body string) replyAction {
	var replyLines []string
	inReply := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.ToLower(line[3:]))
			if heading == "reply" {
				inReply = true
				continue
			}
			if inReply {
				break
			}
		}
		if inReply {
			replyLines = append(replyLines, line)
		}
	}
	if !inReply {
		return replyAction{kind: replyNone}
	}
	text := strings.TrimSpace(strings.Join(replyLines, "\n"))
	if text == "" {
		return replyAction{kind: replyNone}
	}
	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, "close:"):
		return replyAction{kind: replyClose, value: strings.TrimSpace(text[len("close:"):])}
	case strings.HasPrefix(lower, "synonym:"):
		return replyAction{kind: replySynonym, value: strings.TrimSpace(text[len("synonym:"):])}
	case strings.HasPrefix(lower, "stand by"):
		return replyAction{kind: replyStandBy}
	}
	return replyAction{kind: replyNone}
}

func runRespond(ctx context.Context, out io.Writer, root string, now time.Time) error {
	fs := storage.New(root)
	g, err := fs.LoadGlossary(ctx, storage.RepoID(""))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			fmt.Fprintln(out, "no glossary at .ocp/glossary.md; nothing to respond to")
			return nil
		}
		return fmt.Errorf("load glossary: %w", err)
	}

	refs, err := fs.LoadOpenIssues(ctx, storage.RepoID(""))
	if err != nil {
		return fmt.Errorf("list open issues: %w", err)
	}
	if len(refs) == 0 {
		fmt.Fprintln(out, "no open observations")
		return nil
	}

	var (
		closedCount, glossaryUpdates, skipped int
		actions                               []string
	)
	for _, ref := range refs {
		state, err := fs.LoadIssue(ctx, storage.RepoID(""), ref)
		if err != nil {
			return fmt.Errorf("load %s: %w", ref.Path, err)
		}
		action := parseReply(state.Body)
		switch action.kind {
		case replyNone, replyStandBy:
			skipped++
		case replyClose:
			state.Status = storage.IssueClosed
			state.Updated = now
			state.Body = strings.TrimRight(state.Body, "\n") + "\n\nClosed: " + action.value + "\n"
			if err := fs.RecordIssueState(ctx, storage.RepoID(""), state); err != nil {
				return fmt.Errorf("close %s: %w", ref.Path, err)
			}
			closedCount++
			actions = append(actions, fmt.Sprintf("closed %s: %s", ref.Path, action.value))
		case replySynonym:
			updated := addSynonymTo(&g, state.Canonical, action.value)
			if updated {
				if err := fs.SaveGlossary(ctx, storage.RepoID(""), g); err != nil {
					return fmt.Errorf("save glossary: %w", err)
				}
				glossaryUpdates++
			}
			state.Status = storage.IssueClosed
			state.Updated = now
			note := fmt.Sprintf("Closed: added `%s` as synonym of `%s`.", action.value, state.Canonical)
			if !updated {
				note = fmt.Sprintf("Closed: `%s` was already a synonym of `%s`.", action.value, state.Canonical)
			}
			state.Body = strings.TrimRight(state.Body, "\n") + "\n\n" + note + "\n"
			if err := fs.RecordIssueState(ctx, storage.RepoID(""), state); err != nil {
				return fmt.Errorf("close %s: %w", ref.Path, err)
			}
			closedCount++
			actions = append(actions, fmt.Sprintf("synonym `%s` -> `%s`, closed %s", action.value, state.Canonical, ref.Path))
		}
	}

	fmt.Fprintf(out, "respond: %d open, %d closed, %d glossary updates, %d skipped\n",
		len(refs), closedCount, glossaryUpdates, skipped)

	if closedCount > 0 || glossaryUpdates > 0 {
		var body strings.Builder
		body.WriteString("respond:")
		for _, a := range actions {
			fmt.Fprintf(&body, "\n- %s", a)
		}
		if err := fs.AppendLog(ctx, storage.RepoID(""), storage.LogEntry{
			At:   now,
			Body: body.String(),
		}); err != nil {
			return fmt.Errorf("append log: %w", err)
		}
	}
	return nil
}

// addSynonymTo mutates g in place, adding term to the synonym list of
// the given canonical. Returns true if the glossary changed (canonical
// found and term not already present), false otherwise.
func addSynonymTo(g *storage.Glossary, canonical, term string) bool {
	for i, t := range g.Terms {
		if t.Canonical != canonical {
			continue
		}
		for _, s := range t.Synonyms {
			if s == term {
				return false
			}
		}
		g.Terms[i].Synonyms = append(g.Terms[i].Synonyms, term)
		return true
	}
	return false
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(driftCmd)
	rootCmd.AddCommand(respondCmd)
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ocp: %v\n", err)
		os.Exit(1)
	}
}
