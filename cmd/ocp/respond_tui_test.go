package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csellis/ocp/internal/storage"
)

func TestTUIPrompt_Close(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("c\npedagogical use, not drift\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)

	state := storage.IssueState{
		Ref:          storage.IssueRef{Path: "0001-vocabulary-glossary.md"},
		Term:         "vocabulary",
		Canonical:    "glossary",
		Files:        4,
		Occurrences:  47,
		LastReviewed: fixedNow,
		Body:         "# vocabulary -> glossary\n\nUsed in 4 files (47 occurrences):\n\n- doc.md: 1\n",
	}
	action, err := prompt(state)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyClose || action.value != "pedagogical use, not drift" {
		t.Errorf("got %+v", action)
	}

	rendered := out.String()
	for _, want := range []string{
		"0001-vocabulary-glossary.md",
		"vocabulary -> glossary",
		"4 files / 47 occurrences",
		"reviewed 2026-04-26",
		"[c]lose",
		"[d]etails",
		"reason (empty to cancel):",
		"Actions:", // legend printed once at session start
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q in TUI output:\n%s", want, rendered)
		}
	}
}

func TestTUIPrompt_Synonym(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("s\nnew-term\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)

	state := storage.IssueState{
		Ref:       storage.IssueRef{Path: "0002-x.md"},
		Term:      "x",
		Canonical: "y",
	}
	action, err := prompt(state)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replySynonym || action.value != "new-term" {
		t.Errorf("got %+v", action)
	}
}

func TestTUIPrompt_Help(t *testing.T) {
	// ? reprints the legend, then user picks stand-by.
	in := bufio.NewReader(strings.NewReader("?\nb\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	if _, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	rendered := out.String()
	// Legend prints once at session start AND once on ? — count "Actions:" headings.
	if c := strings.Count(rendered, "Actions:"); c != 2 {
		t.Errorf("expected 'Actions:' to appear twice (start + help), got %d:\n%s", c, rendered)
	}
}

func TestTUIPrompt_Details(t *testing.T) {
	// d shows the body inline, then re-prompts; user then closes.
	in := bufio.NewReader(strings.NewReader("d\nc\nclose after details\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)

	state := storage.IssueState{
		Ref:  storage.IssueRef{Path: "x.md"},
		Term: "x", Canonical: "y",
		Body: "# x -> y\n\nUsed in 2 files (3 occurrences):\n\n- a.md: 1, 2\n- b.md: 5\n",
	}
	action, err := prompt(state)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyClose || action.value != "close after details" {
		t.Errorf("got %+v", action)
	}
	rendered := out.String()
	for _, want := range []string{"# x -> y", "- a.md: 1, 2", "- b.md: 5"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("expected body line %q in details output:\n%s", want, rendered)
		}
	}
}

func TestTUIPrompt_HeaderFallbacks(t *testing.T) {
	// State missing Term, Canonical, Files: header still renders gracefully.
	in := bufio.NewReader(strings.NewReader("n\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	_, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	rendered := out.String()
	for _, want := range []string{"(no term recorded)", "(no canonical recorded)"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing fallback %q:\n%s", want, rendered)
		}
	}
}

func TestTUIPrompt_StandBy(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("b\n"))
	prompt := makeTUIPrompt(in, &bytes.Buffer{}, false)
	action, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyStandBy {
		t.Errorf("got %+v", action)
	}
}

func TestTUIPrompt_Next(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("n\n"))
	prompt := makeTUIPrompt(in, &bytes.Buffer{}, false)
	action, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyNone {
		t.Errorf("got %+v", action)
	}
}

func TestTUIPrompt_Quit(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("q\n"))
	prompt := makeTUIPrompt(in, &bytes.Buffer{}, false)
	_, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if !errors.Is(err, errQuit) {
		t.Errorf("expected errQuit, got %v", err)
	}
}

func TestTUIPrompt_UnknownThenClose(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("zzz\nc\nfine\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	action, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyClose || action.value != "fine" {
		t.Errorf("got %+v", action)
	}
	if !strings.Contains(out.String(), `unknown choice "zzz"`) {
		t.Errorf("expected unknown-choice hint, got:\n%s", out.String())
	}
}

func TestTUIPrompt_EmptyReasonCancels(t *testing.T) {
	// Empty enter at the reason sub-prompt cancels back to the menu;
	// user then picks stand-by instead. Nothing closes.
	in := bufio.NewReader(strings.NewReader("c\n\nb\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	action, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if action.kind != replyStandBy {
		t.Errorf("expected replyStandBy after cancel, got %+v", action)
	}
	if !strings.Contains(out.String(), "(cancelled)") {
		t.Errorf("expected '(cancelled)' marker, got:\n%s", out.String())
	}
}

func TestTUIPrompt_EOFMeansQuit(t *testing.T) {
	in := bufio.NewReader(strings.NewReader(""))
	prompt := makeTUIPrompt(in, &bytes.Buffer{}, false)
	_, err := prompt(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}})
	if !errors.Is(err, errQuit) {
		t.Errorf("expected errQuit on EOF, got %v", err)
	}
}

// End-to-end through runRespond: scripted TUI input drives close on
// the only open observation. The file stays in conversation/ (status
// changes via frontmatter only); LoadOpenIssues no longer returns it.
func TestRunRespond_TUIClose(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}

	in := bufio.NewReader(strings.NewReader("c\npedagogical, not drift\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	if err := runRespond(ctx, &out, root, fixedNow, prompt); err != nil {
		t.Fatalf("runRespond: %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "1 closed") {
		t.Errorf("expected '1 closed' in summary:\n%s", rendered)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatalf("read conversation dir: %v", err)
	}
	files := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected one observation file in conversation/, got %v", files)
	}

	body, err := os.ReadFile(filepath.Join(convDir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Status: closed") {
		t.Errorf("expected Status: closed in frontmatter:\n%s", body)
	}
	if !strings.Contains(string(body), "Closed reason: pedagogical, not drift") {
		t.Errorf("expected closed reason in frontmatter:\n%s", body)
	}
}

// Quit mid-queue: TUI handles the first observation, quits before the
// second. Summary reflects only the first action.
func TestRunRespond_TUIQuitMidway(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Two synonyms, two observations.
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\nthe ubiquitous language too.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}

	// Close the first, quit before the second.
	in := bufio.NewReader(strings.NewReader("c\nfirst\nq\n"))
	var out bytes.Buffer
	prompt := makeTUIPrompt(in, &out, false)
	if err := runRespond(ctx, &out, root, fixedNow, prompt); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "1 closed") {
		t.Errorf("expected '1 closed' (only first observation handled):\n%s", rendered)
	}
	// Summary shows 2 open total, but only 1 closed and 0 skipped because
	// we quit before considering the second.
	if !strings.Contains(rendered, "2 open, 1 closed, 0 glossary updates, 0 skipped") {
		t.Errorf("unexpected summary:\n%s", rendered)
	}
}
