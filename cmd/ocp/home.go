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

	"github.com/csellis/ocp/internal/storage"
)

// runHome is the interactive top-level menu. Prints a status block,
// prompts for an action (scan / drift / respond), dispatches to the
// same runScan / runDrift / runRespond used by the subcommands, then
// loops back to home. Only [q]uit (or EOF) returns.
func runHome(ctx context.Context, root string, out io.Writer, stdin *os.File) error {
	st := stylist{on: isTerminal(stdin) && isTerminal(os.Stdout)}
	in := bufio.NewReader(stdin)

	for {
		now := time.Now().UTC()
		state := readHomeState(ctx, root, now)
		printHomeStatus(out, state, st)

		fmt.Fprintln(out)
		fmt.Fprintln(out, st.bold("What now?"))
		fmt.Fprintln(out, "  [s] scan        seed or reload the glossary")
		fmt.Fprintln(out, "  [d] drift       look for drift in the working tree")
		fmt.Fprintln(out, "  [r] respond     walk open observations")
		fmt.Fprintln(out, "  [?] help        more detail")
		fmt.Fprintln(out, "  [q] quit")

		if err := homeChoice(ctx, root, out, in, st, now); err != nil {
			if errors.Is(err, errHomeQuit) {
				return nil
			}
			return err
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, st.dim("---"))
		fmt.Fprintln(out)
	}
}

// errHomeQuit signals "user asked to leave the home menu." Distinct
// from a real error so the home loop can return cleanly.
var errHomeQuit = errors.New("home: quit")

// homeChoice reads and dispatches one menu choice. Returns errHomeQuit
// when the user picks q, EOF on stdin, or any real action error.
// Re-prompts on unknown input.
func homeChoice(ctx context.Context, root string, out io.Writer, in *bufio.Reader, st stylist, now time.Time) error {
	for {
		fmt.Fprint(out, "  > ")
		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return errHomeQuit
			}
			return fmt.Errorf("read input: %w", err)
		}
		choice := strings.ToLower(strings.TrimSpace(line))
		switch choice {
		case "s", "scan":
			fmt.Fprintln(out)
			return runScan(ctx, out, root, now)
		case "d", "drift":
			fmt.Fprintln(out)
			return runDrift(ctx, out, root, now)
		case "r", "respond":
			fmt.Fprintln(out)
			return runRespond(ctx, out, root, now, makeTUIPrompt(in, out, st.on))
		case "?", "h", "help":
			fmt.Fprintln(out)
			printHomeHelp(out, st)
			continue
		case "q", "quit", "":
			return errHomeQuit
		default:
			fmt.Fprintf(out, "  unknown choice %q; try s, d, r, ?, q\n", choice)
		}
	}
}

// homeState is the at-a-glance summary the home menu shows. Pulled
// from storage on demand; cheap (a glossary read, a directory list,
// a log stat).
type homeState struct {
	HasGlossary    bool
	GlossaryTerms  int
	OpenObs        int
	OldestReviewed time.Time // zero if no open observations
	LogEntries     int
	ShipName       string
}

func readHomeState(ctx context.Context, root string, now time.Time) homeState {
	fs := storage.New(root)
	var s homeState

	if g, err := fs.LoadGlossary(ctx, storage.RepoID("")); err == nil {
		s.HasGlossary = true
		s.GlossaryTerms = len(g.Terms)
	}

	if refs, err := fs.LoadOpenIssues(ctx, storage.RepoID("")); err == nil {
		s.OpenObs = len(refs)
		for _, ref := range refs {
			st, err := fs.LoadIssue(ctx, storage.RepoID(""), ref)
			if err != nil {
				continue
			}
			if st.LastReviewed.IsZero() {
				continue
			}
			if s.OldestReviewed.IsZero() || st.LastReviewed.Before(s.OldestReviewed) {
				s.OldestReviewed = st.LastReviewed
			}
		}
	}

	if data, err := os.ReadFile(root + "/.ocp/log.md"); err == nil {
		// Count "## " headings; cheap heuristic for entry count.
		s.LogEntries = strings.Count(string(data), "\n## ")
		if strings.HasPrefix(string(data), "## ") {
			s.LogEntries++
		}
	}

	return s
}

func printHomeStatus(out io.Writer, s homeState, st stylist) {
	fmt.Fprintln(out, st.boldCyan("ocp"))
	fmt.Fprintln(out)

	if !s.HasGlossary {
		printWelcome(out, st)
		return
	}

	fmt.Fprintln(out, st.bold("State:"))
	fmt.Fprintf(out, "  glossary    %d terms\n", s.GlossaryTerms)
	fmt.Fprintf(out, "  open obs    %d", s.OpenObs)
	if s.OpenObs > 0 && !s.OldestReviewed.IsZero() {
		fmt.Fprintf(out, "  %s", st.dim(fmt.Sprintf("(oldest reviewed %s)", humanizeAge(s.OldestReviewed))))
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  log         %d %s\n", s.LogEntries, pluralize("entry", s.LogEntries))
}

// printHomeHelp re-shows the home action menu with longer descriptions.
// Triggered by ? / h / help at the home prompt; useful when the menu
// has scrolled off or the user wants the long version.
func printHomeHelp(out io.Writer, st stylist) {
	fmt.Fprintln(out, st.bold("Actions:"))
	fmt.Fprintln(out, "  [s] scan        read or seed .ocp/glossary.md (start here on a fresh repo)")
	fmt.Fprintln(out, "  [d] drift       walk the working tree, file an observation per drift event")
	fmt.Fprintln(out, "  [r] respond     walk open observations and decide on each (interactive)")
	fmt.Fprintln(out, "  [?] help        show this list")
	fmt.Fprintln(out, "  [q] quit        leave OCP")
	fmt.Fprintln(out)
	fmt.Fprintln(out, st.dim("Background: README.md, docs/THESIS.md, AGENTS.md."))
}

// printWelcome runs on the first home open in a repo (no .ocp/glossary.md
// yet). Sets context for someone who has just installed OCP and run it
// for the first time. Once scan creates the glossary, this never shows
// again — no nag.
func printWelcome(out io.Writer, st stylist) {
	fmt.Fprintln(out, "  "+st.bold("Welcome."))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  OCP watches a codebase for drift in its ubiquitous language: the")
	fmt.Fprintln(out, "  canonical names a team has agreed on for the concepts in their domain.")
	fmt.Fprintln(out, "  It does not write code. It surfaces single observations and updates")
	fmt.Fprintln(out, "  its own .ocp/ state. Speak rarely; speak deliberately.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  "+st.bold("Three actions:"))
	fmt.Fprintln(out, "    "+st.cyan("scan")+"     read or seed .ocp/glossary.md (start here)")
	fmt.Fprintln(out, "    "+st.cyan("drift")+"    walk the working tree, file an observation per drift event")
	fmt.Fprintln(out, "    "+st.cyan("respond")+"  walk open observations and decide on each")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  "+st.dim("Background: README.md, docs/THESIS.md, AGENTS.md."))
}

// humanizeAge returns a coarse "N days ago" for the home status. Times
// are rounded to whole days because finer precision is noise here.
func humanizeAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", int(d/(24*time.Hour)))
	}
}
