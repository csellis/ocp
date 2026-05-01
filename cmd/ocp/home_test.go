package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunHome_FreshRepoShowsWelcome(t *testing.T) {
	root := t.TempDir()
	in := openStdin(t, "q\n")
	defer in.Close()
	var out bytes.Buffer
	if err := runHome(context.Background(), root, &out, in); err != nil {
		t.Fatalf("runHome: %v", err)
	}
	rendered := out.String()
	for _, want := range []string{
		"Welcome.",
		"watches a codebase for drift",
		"Three actions:",
		"scan",
		"drift",
		"respond",
		"README.md",
		"What now?",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q in home output:\n%s", want, rendered)
		}
	}
}

func TestRunHome_AfterScan(t *testing.T) {
	root := t.TempDir()
	if err := runScan(context.Background(), &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	in := openStdin(t, "q\n")
	defer in.Close()
	var out bytes.Buffer
	if err := runHome(context.Background(), root, &out, in); err != nil {
		t.Fatalf("runHome: %v", err)
	}
	rendered := out.String()
	for _, want := range []string{"State:", "glossary    7 terms", "open obs    0", "log         1 entry"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q in home output:\n%s", want, rendered)
		}
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
		{"day", 2, "days"}, // vowel+y stays +s
	}
	for _, tc := range cases {
		if got := pluralize(tc.in, tc.n); got != tc.want {
			t.Errorf("pluralize(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestRunHome_DispatchToScan(t *testing.T) {
	root := t.TempDir()
	in := openStdin(t, "s\n")
	defer in.Close()
	var out bytes.Buffer
	if err := runHome(context.Background(), root, &out, in); err != nil {
		t.Fatalf("runHome: %v", err)
	}
	if !strings.Contains(out.String(), "wrote new glossary") {
		t.Errorf("expected scan output (wrote new glossary), got:\n%s", out.String())
	}
}

func TestRunHome_HelpReprintsActions(t *testing.T) {
	root := t.TempDir()
	if err := runScan(context.Background(), &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	in := openStdin(t, "?\nq\n")
	defer in.Close()
	var out bytes.Buffer
	if err := runHome(context.Background(), root, &out, in); err != nil {
		t.Fatalf("runHome: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "Actions:") {
		t.Errorf("expected 'Actions:' header from help:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[?] help") {
		t.Errorf("expected '[?] help' line:\n%s", rendered)
	}
}

func TestRunHome_UnknownChoiceThenQuit(t *testing.T) {
	root := t.TempDir()
	in := openStdin(t, "x\nq\n")
	defer in.Close()
	var out bytes.Buffer
	if err := runHome(context.Background(), root, &out, in); err != nil {
		t.Fatalf("runHome: %v", err)
	}
	if !strings.Contains(out.String(), `unknown choice "x"`) {
		t.Errorf("expected unknown-choice hint, got:\n%s", out.String())
	}
}

// openStdin returns an *os.File backed by a temp file containing the
// scripted input. We use a temp file rather than an io.Pipe because
// runHome takes *os.File (it inspects with isTerminal); a temp file is
// not a TTY, which is exactly the test mode we want (color off in
// home_test through stylist's isTerminal check on stdout).
func openStdin(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stdin: %v", err)
	}
	return f
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
