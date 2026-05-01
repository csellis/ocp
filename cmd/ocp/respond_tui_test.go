package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/csellis/ocp/internal/storage"
)

func TestRespondModel_Close(t *testing.T) {
	state := storage.IssueState{
		Ref:          storage.IssueRef{Path: "0001-vocabulary-glossary.md"},
		Term:         "vocabulary",
		Canonical:    "glossary",
		Files:        4,
		Occurrences:  47,
		LastReviewed: fixedNow,
		Body:         "# vocabulary -> glossary\n\nUsed in 4 files (47 occurrences):\n\n- doc.md: 1\n",
	}
	m := drive(newRespondModel(state, false), rune2key('c'))
	if m.mode != modeReasonInput {
		t.Fatalf("expected modeReasonInput after 'c', got %v", m.mode)
	}
	m = typeText(m, "pedagogical use, not drift")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got, err := extractAction(next.(respondModel))
	if err != nil || got.kind != replyClose || got.value != "pedagogical use, not drift" {
		t.Errorf("extractAction: got (%+v, %v)", got, err)
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit cmd after submitting reason")
	}
	m = next.(respondModel)

	header := respondHeader(state, m.styles)
	for _, want := range []string{
		"0001-vocabulary-glossary.md",
		"vocabulary",
		"glossary",
		"4 files / 47 occurrences",
		"reviewed 2026-04-26",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("missing %q in header:\n%s", want, header)
		}
	}
}

func TestRespondModel_Synonym(t *testing.T) {
	state := storage.IssueState{Ref: storage.IssueRef{Path: "0002-x.md"}, Term: "x", Canonical: "y"}
	m := drive(newRespondModel(state, false), rune2key('s'))
	if m.mode != modeSynonymInput {
		t.Fatalf("expected modeSynonymInput, got %v", m.mode)
	}
	m = typeText(m, "new-term")
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.action.kind != replySynonym || m.action.value != "new-term" {
		t.Errorf("got %+v", m.action)
	}
}

func TestRespondModel_HelpToggle(t *testing.T) {
	m := newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false)
	if strings.Contains(m.View(), "Actions:") {
		t.Errorf("legend should not appear inside the model until ?")
	}
	m = drive(m, rune2key('?'))
	if !strings.Contains(m.View(), "Actions:") {
		t.Errorf("expected Actions: header after ?:\n%s", m.View())
	}
	m = drive(m, rune2key('?'))
	if strings.Contains(m.View(), "Actions:") {
		t.Errorf("expected help hidden after second ?")
	}
}

func TestRespondModel_DetailsToggle(t *testing.T) {
	state := storage.IssueState{
		Ref:  storage.IssueRef{Path: "x.md"},
		Term: "x", Canonical: "y",
		Body: "# x -> y\n\nUsed in 2 files (3 occurrences):\n\n- a.md: 1, 2\n- b.md: 5\n",
	}
	m := drive(newRespondModel(state, false), rune2key('d'))
	rendered := m.View()
	for _, want := range []string{"# x -> y", "- a.md: 1, 2", "- b.md: 5"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("expected body line %q in details:\n%s", want, rendered)
		}
	}
	// d again hides details.
	m = drive(m, rune2key('d'))
	if strings.Contains(m.View(), "- a.md: 1, 2") {
		t.Errorf("expected details hidden after second d")
	}
}

func TestRespondModel_HeaderFallbacks(t *testing.T) {
	m := newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false)
	rendered := m.View()
	for _, want := range []string{"(no term recorded)", "(no canonical recorded)"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing fallback %q:\n%s", want, rendered)
		}
	}
}

func TestRespondModel_StandBy(t *testing.T) {
	m := newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false)
	next, cmd := m.Update(rune2key('b'))
	got, err := extractAction(next.(respondModel))
	if err != nil || got.kind != replyStandBy {
		t.Errorf("extractAction: got (%+v, %v); want (replyStandBy, nil)", got, err)
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit cmd")
	}
}

func TestRespondModel_Next(t *testing.T) {
	m := newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false)
	next, cmd := m.Update(rune2key('n'))
	got, err := extractAction(next.(respondModel))
	if err != nil || got.kind != replyNone {
		t.Errorf("extractAction: got (%+v, %v); want (replyNone, nil)", got, err)
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit cmd on next")
	}
}

func TestRespondModel_QuitFromMenu(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		rune2key('q'),
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	} {
		m := drive(newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false), key)
		if !m.quitting {
			t.Errorf("expected quitting=true on %v", key)
		}
		if _, err := extractAction(m); !errors.Is(err, errQuit) {
			t.Errorf("expected errQuit for %v, got %v", key, err)
		}
	}
}

func TestRespondModel_UnknownKeyShowsFlash(t *testing.T) {
	m := drive(newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false), rune2key('z'))
	if !strings.Contains(m.View(), `unknown choice "z"`) {
		t.Errorf("expected unknown-choice flash:\n%s", m.View())
	}
	// Next valid key clears the flash and acts normally.
	m = drive(m, rune2key('b'))
	if m.action.kind != replyStandBy {
		t.Errorf("expected stand-by after recovering from unknown key, got %+v", m.action)
	}
}

func TestRespondModel_EmptyReasonCancels(t *testing.T) {
	m := drive(newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false), rune2key('c'))
	if m.mode != modeReasonInput {
		t.Fatalf("expected modeReasonInput")
	}
	// Empty enter cancels back to menu with a flash.
	m = drive(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeMenu {
		t.Errorf("expected back to modeMenu, got %v", m.mode)
	}
	if !strings.Contains(m.View(), "(cancelled)") {
		t.Errorf("expected '(cancelled)' flash, got:\n%s", m.View())
	}
	// Stand-by from menu works after cancel.
	m = drive(m, rune2key('b'))
	if m.action.kind != replyStandBy {
		t.Errorf("expected stand-by, got %+v", m.action)
	}
}

func TestRespondModel_EscFromInputCancels(t *testing.T) {
	m := drive(newRespondModel(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}}, false), rune2key('s'))
	m = typeText(m, "partial")
	m = drive(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeMenu {
		t.Errorf("expected back to modeMenu after esc, got %v", m.mode)
	}
	if m.action.kind != replyNone {
		t.Errorf("expected no action recorded, got %+v", m.action)
	}
}

func TestRespondHeader_DimDensityOnZero(t *testing.T) {
	// Files == 0 means the density block is omitted from the header.
	header := respondHeader(storage.IssueState{Ref: storage.IssueRef{Path: "x.md"}, Term: "t", Canonical: "c"}, newHomeStyles(false))
	if strings.Contains(header, "occurrence") {
		t.Errorf("expected density omitted when Files == 0:\n%s", header)
	}
}

// fakePromptScript drives runRespond without any TUI: it returns
// queued actions, one per call. errors are returned verbatim.
type fakePromptScript struct {
	actions []replyAction
	errs    []error
	i       int
}

func (f *fakePromptScript) prompt(_ storage.IssueState) (replyAction, error) {
	if f.i >= len(f.actions) {
		return replyAction{}, errQuit
	}
	a, err := f.actions[f.i], f.errs[f.i]
	f.i++
	return a, err
}

// End-to-end through runRespond: a fake prompter returns close on the
// only open observation. The file stays in conversation/ (status changes
// via frontmatter only); LoadOpenIssues no longer returns it.
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

	script := &fakePromptScript{
		actions: []replyAction{{kind: replyClose, value: "pedagogical, not drift"}},
		errs:    []error{nil},
	}
	var out bytes.Buffer
	if err := runRespond(ctx, &out, root, fixedNow, script.prompt); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(out.String(), "1 closed") {
		t.Errorf("expected '1 closed' in summary:\n%s", out.String())
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
		t.Fatalf("expected one observation file, got %v", files)
	}
	body, err := os.ReadFile(filepath.Join(convDir, files[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Status: closed") {
		t.Errorf("expected Status: closed:\n%s", body)
	}
	if !strings.Contains(string(body), "Closed reason: pedagogical, not drift") {
		t.Errorf("expected closed reason in frontmatter:\n%s", body)
	}
}

// Quit mid-queue: prompter returns close on first, errQuit on second.
// Summary reflects only the first action.
func TestRunRespond_TUIQuitMidway(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\nthe ubiquitous language too.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}
	script := &fakePromptScript{
		actions: []replyAction{{kind: replyClose, value: "first"}, {}},
		errs:    []error{nil, errQuit},
	}
	var out bytes.Buffer
	if err := runRespond(ctx, &out, root, fixedNow, script.prompt); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(out.String(), "2 open, 1 closed, 0 glossary updates, 0 skipped") {
		t.Errorf("unexpected summary:\n%s", out.String())
	}
}

// drive runs Update with a single message and panics on cmd execution
// (we only care about state transitions and View output, not the
// async cmd plumbing). Returns the new model in concrete type.
func drive(m respondModel, msg tea.Msg) respondModel {
	next, _ := m.Update(msg)
	return next.(respondModel)
}

// typeText feeds a string into the focused textinput one rune at a time.
// Mirrors what bubbletea would deliver from a real keystroke stream.
func typeText(m respondModel, s string) respondModel {
	for _, r := range s {
		m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}
