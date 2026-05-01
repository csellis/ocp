package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/csellis/ocp/internal/storage"
)

// makeTUIPrompt returns a prompter that walks one observation at a time
// in the terminal. The closure captures the input reader, writer, and
// color-on flag so runRespond can drive a sequence of prompts without
// re-allocating per observation. The legend prints once on the first
// call; per-observation prompts use the compact menu.
//
// Tests inject any io.Reader / io.Writer pair to drive scripted input
// and pass color=false to keep assertions ANSI-free.
func makeTUIPrompt(in *bufio.Reader, out io.Writer, color bool) prompter {
	st := stylist{on: color}
	first := true
	return func(state storage.IssueState) (replyAction, error) {
		if first {
			printLegend(out, st)
			first = false
		}
		fmt.Fprintln(out)
		printHeader(out, state, st)

		for {
			fmt.Fprint(out, "  > ")
			line, err := in.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return replyAction{}, errQuit
				}
				return replyAction{}, fmt.Errorf("read input: %w", err)
			}
			choice := strings.ToLower(strings.TrimSpace(line))
			switch choice {
			case "c", "close":
				reason, ok, err := readFollowup(in, out, "  reason (empty to cancel): ")
				if err != nil {
					return replyAction{}, err
				}
				if !ok {
					fmt.Fprintln(out, st.dim("  (cancelled)"))
					continue
				}
				return replyAction{kind: replyClose, value: reason}, nil
			case "s", "synonym":
				term, ok, err := readFollowup(in, out, "  term (empty to cancel): ")
				if err != nil {
					return replyAction{}, err
				}
				if !ok {
					fmt.Fprintln(out, st.dim("  (cancelled)"))
					continue
				}
				return replyAction{kind: replySynonym, value: term}, nil
			case "b", "stand by", "standby":
				return replyAction{kind: replyStandBy}, nil
			case "d", "details":
				printDetails(out, state, st)
				continue
			case "?", "h", "help":
				fmt.Fprintln(out)
				printLegend(out, st)
				continue
			case "n", "next", "":
				return replyAction{kind: replyNone}, nil
			case "q", "quit":
				return replyAction{}, errQuit
			default:
				fmt.Fprintf(out, "  unknown choice %q; try c, s, b, d, ?, n, q\n", choice)
			}
		}
	}
}

// printLegend explains each menu choice in plain words. Shown once per
// session so first-time users learn what "close" actually does.
func printLegend(out io.Writer, st stylist) {
	fmt.Fprintln(out, st.bold("Actions:"))
	fmt.Fprintln(out, "  [c]lose      this isn't drift; archive with a reason")
	fmt.Fprintln(out, "  [s]ynonym    add the term as a glossary synonym; archive")
	fmt.Fprintln(out, "  [b] stand by leave open; revisit later")
	fmt.Fprintln(out, "  [d]etails    show the full citation list")
	fmt.Fprintln(out, "  [n]ext       skip without changing anything")
	fmt.Fprintln(out, "  [q]uit       stop reviewing")
}

// printHeader is the at-a-glance one-liner shown above the menu. Reads
// structured fields from frontmatter; no body parsing.
func printHeader(out io.Writer, s storage.IssueState, st stylist) {
	term := s.Term
	if term == "" {
		term = "(no term recorded)"
	}
	canonical := s.Canonical
	if canonical == "" {
		canonical = "(no canonical recorded)"
	}
	density := ""
	if s.Files > 0 {
		density = fmt.Sprintf("  %s",
			st.dim(fmt.Sprintf("%d %s / %d %s",
				s.Files, pluralize("file", s.Files),
				s.Occurrences, pluralize("occurrence", s.Occurrences))))
	}
	reviewed := ""
	if !s.LastReviewed.IsZero() {
		reviewed = "  " + st.dim("reviewed "+s.LastReviewed.UTC().Format("2006-01-02"))
	}
	fmt.Fprintf(out, "%s   %s %s %s%s%s\n",
		st.dim(s.Ref.Path),
		st.boldCyan(term),
		st.dim("->"),
		st.bold(canonical),
		density,
		reviewed,
	)
	fmt.Fprintln(out, st.dim("  [c]lose  [s]ynonym  [b] stand by  [d]etails  [?]help  [n]ext  [q]uit"))
}

func printDetails(out io.Writer, s storage.IssueState, st stylist) {
	fmt.Fprintln(out)
	body := strings.TrimRight(s.Body, "\n")
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(out, "    %s\n", st.dim(line))
	}
}

// readFollowup prompts for a follow-up value. ok=false means the user
// cancelled (empty input or EOF). Errors are real I/O failures.
func readFollowup(in *bufio.Reader, out io.Writer, label string) (string, bool, error) {
	fmt.Fprint(out, label)
	line, err := in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, fmt.Errorf("read input: %w", err)
	}
	v := strings.TrimSpace(line)
	if v == "" {
		return "", false, nil
	}
	return v, true, nil
}
