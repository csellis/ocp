package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHomeModel_FreshRepoShowsWelcome(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: false}, false)
	out := m.View()
	for _, want := range []string{
		"Welcome.",
		"watches a codebase for drift",
		"Three actions:",
		"scan", "drift", "respond",
		"README.md",
		"What now?",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in fresh-repo view:\n%s", want, out)
		}
	}
}

func TestHomeModel_AfterScanShowsState(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true, GlossaryTerms: 7, OpenObs: 0, LogEntries: 1}, false)
	out := m.View()
	for _, want := range []string{
		"State:",
		"glossary    7 terms",
		"open obs    0",
		"log         1 entry",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in state view:\n%s", want, out)
		}
	}
}

func TestHomeModel_QuitOnQ(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	next, cmd := m.Update(rune2key('q'))
	if next.(homeModel).choice != choiceQuit {
		t.Errorf("expected choiceQuit, got %v", next.(homeModel).choice)
	}
	if cmd == nil {
		t.Errorf("expected non-nil cmd (tea.Quit)")
	}
}

func TestHomeModel_QuitOnEsc(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(homeModel).choice != choiceQuit {
		t.Errorf("expected choiceQuit on esc")
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit on esc")
	}
}

func TestHomeModel_QuitOnCtrlC(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if next.(homeModel).choice != choiceQuit {
		t.Errorf("expected choiceQuit on ctrl+c")
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit on ctrl+c")
	}
}

func TestHomeModel_LetterShortcutPicksScan(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	next, cmd := m.Update(rune2key('s'))
	if next.(homeModel).choice != choiceScan {
		t.Errorf("expected choiceScan, got %v", next.(homeModel).choice)
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit cmd")
	}
}

func TestHomeModel_ArrowKeysAndEnter(t *testing.T) {
	var m tea.Model = newHomeModel(homeState{HasGlossary: true}, false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.(homeModel).cursor != 2 {
		t.Errorf("expected cursor=2 after two downs, got %d", m.(homeModel).cursor)
	}
	// One more down should clamp at the last item, not wrap.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.(homeModel).cursor != 2 {
		t.Errorf("cursor should clamp at last item, got %d", m.(homeModel).cursor)
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.(homeModel).choice != choiceRespond {
		t.Errorf("expected choiceRespond, got %v", m.(homeModel).choice)
	}
	if cmd == nil {
		t.Errorf("expected tea.Quit on enter")
	}
}

func TestHomeModel_HelpToggle(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	if strings.Contains(m.View(), "Actions:") {
		t.Errorf("help should be hidden initially:\n%s", m.View())
	}
	next, _ := m.Update(rune2key('?'))
	if !strings.Contains(next.(homeModel).View(), "Actions:") {
		t.Errorf("expected Actions: header after ?:\n%s", next.(homeModel).View())
	}
	next2, _ := next.(homeModel).Update(rune2key('?'))
	if strings.Contains(next2.(homeModel).View(), "Actions:") {
		t.Errorf("expected help hidden after second ?:\n%s", next2.(homeModel).View())
	}
}

func TestHomeModel_UnknownKeyIsNoop(t *testing.T) {
	m := newHomeModel(homeState{HasGlossary: true}, false)
	next, cmd := m.Update(rune2key('x'))
	if next.(homeModel).choice != choiceNone {
		t.Errorf("expected choiceNone after unknown key, got %v", next.(homeModel).choice)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for unknown key")
	}
}

func TestDispatchHome_RunsScan(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := dispatchHome(context.Background(), root, &out, strings.NewReader(""), choiceScan, fixedNow, false)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out.String(), "wrote new glossary") {
		t.Errorf("expected scan output, got:\n%s", out.String())
	}
}

func TestDispatchHome_RunsDrift(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var out bytes.Buffer
	err := dispatchHome(ctx, root, &out, strings.NewReader(""), choiceDrift, fixedNow, false)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out.String(), "no drift detected") {
		t.Errorf("expected drift output, got:\n%s", out.String())
	}
}

func TestDispatchHome_RunsRespondNoIssues(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var out bytes.Buffer
	err := dispatchHome(ctx, root, &out, strings.NewReader(""), choiceRespond, fixedNow, false)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(out.String(), "no open observations") {
		t.Errorf("expected respond output, got:\n%s", out.String())
	}
}

func TestPluralize(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"entry", 1, "entry"},
		{"entry", 2, "entries"},
		{"file", 1, "file"},
		{"file", 2, "files"},
		{"occurrence", 1, "occurrence"},
		{"occurrence", 0, "occurrences"},
		{"day", 2, "days"},
	}
	for _, tc := range cases {
		if got := pluralize(tc.in, tc.n); got != tc.want {
			t.Errorf("pluralize(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	cases := []struct {
		ageHours int
		want     string
	}{
		{0, "today"},
		{12, "today"},
		{30, "yesterday"},
		{72, "3 days ago"},
	}
	for _, tc := range cases {
		got := humanizeAge(time.Now().Add(-time.Duration(tc.ageHours) * time.Hour))
		if got != tc.want {
			t.Errorf("humanizeAge(%dh) = %q, want %q", tc.ageHours, got, tc.want)
		}
	}
}

// rune2key wraps a single printable rune in the KeyMsg shape bubbletea
// emits for letter keystrokes ("s", "?", etc.).
func rune2key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
