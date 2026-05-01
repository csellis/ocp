package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/csellis/ocp/internal/storage"
)

// respondFromFile is set by the --from-file flag. When true, runRespond
// always uses the file-Reply parser; when false, it picks a prompt based
// on whether stdin is a TTY.
var respondFromFile bool

var respondCmd = &cobra.Command{
	Use:   "respond",
	Short: "Walk open observations and act on each (interactive by default).",
	Long: `Respond walks every open observation in .ocp/conversation/.

Two front-ends share the same action dispatch:

  Interactive (default when stdin is a terminal):
    Walks each open observation, prints a one-line summary, and
    prompts for an action: [c]lose, [s]ynonym, [b] stand by,
    [n]ext, [q]uit. Close and synonym ask for a follow-up value.

  File (--from-file, or automatic when stdin is not a terminal):
    Reads a "## Reply" section the maintainer has appended to the
    body. Recognized formats (case-insensitive):
      close: <reason>      Close with a closing note.
      synonym: <term>      Add term as a synonym of the canonical,
                           then close.
      stand by             Leave open. No state change.
    Observations without a Reply (or with an unrecognized one) are
    skipped silently.

Both front-ends apply the same close/synonym semantics. Glossary
updates write atomically; the .ocp/log.md gets one entry summarizing
the run when state changed.

This v0.1 surface is a keyword parser. v0.2 replaces it with the
LLM-driven conversation loop the architecture describes.

Exit codes:
  0  success (including TUI quit before processing all observations)
  1  unrecoverable error (unreadable observation, glossary write failure)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cwd: %w", err)
		}
		prompt := pickPrompt(cmd.OutOrStdout(), cmd.ErrOrStderr())
		return runRespond(cmd.Context(), cmd.OutOrStdout(), cwd, time.Now().UTC(), prompt)
	},
}

// pickPrompt decides which front-end to use. --from-file always wins;
// otherwise we look at stdin to decide between TUI and a non-interactive
// file fallback (so piped or CI runs don't hang waiting for input).
func pickPrompt(out, errOut io.Writer) prompter {
	if respondFromFile {
		return filePrompt
	}
	if isTerminal(os.Stdin) {
		return makeTUIPrompt(bufio.NewReader(os.Stdin), out, isTerminal(os.Stdout))
	}
	fmt.Fprintln(errOut, "respond: stdin is not a terminal, falling back to file-Reply mode")
	return filePrompt
}

// isTerminal returns true if f looks like a real TTY. Uses Stat rather
// than golang.org/x/term to keep the dep tree narrow.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
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

// errQuit signals that the TUI user asked to stop processing the queue.
// runRespond catches it, prints the partial summary, returns nil to the
// caller. Not an error from the user's point of view; the sentinel just
// breaks the loop.
var errQuit = errors.New("respond: user quit")

// prompter produces the action for one observation. The TUI implementation
// shows a menu; the file implementation parses an embedded Reply section.
type prompter func(state storage.IssueState) (replyAction, error)

// filePrompt is the original v0.1 behavior: parse a `## Reply` section
// from the observation body. Returns replyNone if there is no Reply or
// if the keyword is unrecognized; the run silently skips those.
func filePrompt(state storage.IssueState) (replyAction, error) {
	return parseReply(state.Body), nil
}

// parseReply scans an observation body for a "## Reply" section and
// extracts the recognized intent.
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

func runRespond(ctx context.Context, out io.Writer, root string, now time.Time, prompt prompter) error {
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
		state.Ref = ref // LoadIssue sets this; defensive

		action, err := prompt(state)
		if err != nil {
			if errors.Is(err, errQuit) {
				break
			}
			return fmt.Errorf("prompt %s: %w", ref.Path, err)
		}

		// Reaching this point means the user (or the file parser) gave us
		// an answer for this observation, even if the answer is "skip."
		// That counts as a review; bump LastReviewed and persist.
		state.LastReviewed = now

		switch action.kind {
		case replyNone, replyStandBy:
			skipped++
			if err := fs.RecordIssueState(ctx, storage.RepoID(""), state); err != nil {
				return fmt.Errorf("update %s: %w", ref.Path, err)
			}
		case replyClose:
			if err := applyClose(ctx, fs, &state, action.value); err != nil {
				return err
			}
			closedCount++
			actions = append(actions, fmt.Sprintf("closed %s: %s", ref.Path, action.value))
		case replySynonym:
			updated, err := applySynonym(ctx, fs, &g, &state, action.value)
			if err != nil {
				return err
			}
			if updated {
				glossaryUpdates++
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

// applyClose marks state closed and persists. Closure reason lives in
// frontmatter (state.ClosedReason); body is left alone. LastReviewed
// is bumped by the caller before this runs.
func applyClose(ctx context.Context, fs *storage.Filesystem, state *storage.IssueState, reason string) error {
	state.Status = storage.IssueClosed
	state.ClosedReason = reason
	if err := fs.RecordIssueState(ctx, storage.RepoID(""), *state); err != nil {
		return fmt.Errorf("close %s: %w", state.Ref.Path, err)
	}
	return nil
}

// applySynonym updates the glossary if needed, then closes the
// observation with a frontmatter note describing the action. Returns
// whether the glossary actually changed.
func applySynonym(ctx context.Context, fs *storage.Filesystem, g *storage.Glossary, state *storage.IssueState, term string) (bool, error) {
	updated := addSynonymTo(g, state.Canonical, term)
	if updated {
		if err := fs.SaveGlossary(ctx, storage.RepoID(""), *g); err != nil {
			return false, fmt.Errorf("save glossary: %w", err)
		}
	}
	state.Status = storage.IssueClosed
	if updated {
		state.ClosedReason = fmt.Sprintf("added `%s` as synonym of `%s`", term, state.Canonical)
	} else {
		state.ClosedReason = fmt.Sprintf("`%s` was already a synonym of `%s`", term, state.Canonical)
	}
	if err := fs.RecordIssueState(ctx, storage.RepoID(""), *state); err != nil {
		return updated, fmt.Errorf("close %s: %w", state.Ref.Path, err)
	}
	return updated, nil
}

func init() {
	respondCmd.Flags().BoolVar(&respondFromFile, "from-file", false,
		"force file-Reply parsing instead of the interactive TUI")
}

// addSynonymTo mutates g in place, adding term to the synonym list of
// the given canonical. Returns true if the glossary changed.
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
