package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// repoID used in every test; Filesystem ignores it but the interface requires it.
const testRepo = RepoID("local")

func TestLoadGlossary_Missing(t *testing.T) {
	fs := New(t.TempDir())
	_, err := fs.LoadGlossary(context.Background(), testRepo)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGlossaryRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   Glossary
	}{
		{
			name: "single term, no synonyms",
			in: Glossary{Terms: []Term{
				{Canonical: "drift", Definition: "movement of a canonical term toward synonymy, ambiguity, or vagueness."},
			}},
		},
		{
			name: "multiple terms with synonyms",
			in: Glossary{Terms: []Term{
				{Canonical: "eval", Definition: "an evaluation pass against the corpus.", Synonyms: []string{"assessment", "verification"}},
				{Canonical: "scout", Definition: "the cheap-stage detector. zero LLM calls."},
				{Canonical: "ship-name", Definition: "the deployed instance's banks-style name.", Synonyms: []string{"instance-name"}},
			}},
		},
		{
			name: "empty glossary",
			in:   Glossary{Terms: []Term{}},
		},
		{
			name: "definition contains markdown",
			in: Glossary{Terms: []Term{
				{Canonical: "observation", Definition: "an OCP-authored issue body.\n\nMultiple paragraphs are allowed and must round-trip cleanly."},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := New(t.TempDir())
			ctx := context.Background()
			if err := fs.SaveGlossary(ctx, testRepo, tc.in); err != nil {
				t.Fatalf("SaveGlossary: %v", err)
			}
			got, err := fs.LoadGlossary(ctx, testRepo)
			if err != nil {
				t.Fatalf("LoadGlossary: %v", err)
			}
			if !glossariesEqual(got, tc.in) {
				t.Errorf("round-trip mismatch\nwant: %#v\n got: %#v", tc.in, got)
			}
		})
	}
}

func TestSaveGlossary_CreatesOcpDir(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	if err := fs.SaveGlossary(context.Background(), testRepo, Glossary{Terms: []Term{{Canonical: "x", Definition: "y"}}}); err != nil {
		t.Fatalf("SaveGlossary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ocp", "glossary.md")); err != nil {
		t.Fatalf("expected .ocp/glossary.md to exist: %v", err)
	}
}

// TestAtomicWrite_FileMode pins the mode of files written through atomicWrite
// to 0o644. os.CreateTemp defaults to 0o600 which is too restrictive for
// project-level files (glossary, log, observations all want world-readable
// for ergonomic editor and tool access).
func TestAtomicWrite_FileMode(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()

	if err := fs.SaveGlossary(ctx, testRepo, Glossary{Terms: []Term{{Canonical: "x", Definition: "y"}}}); err != nil {
		t.Fatalf("SaveGlossary: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, ".ocp", "glossary.md"))
	if err != nil {
		t.Fatalf("stat glossary: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("glossary mode: want 0o644, got %#o", got)
	}
}

func TestAppendLog(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()

	entries := []LogEntry{
		{At: mustTime(t, "2026-04-26T10:00:00Z"), Body: "first observation"},
		{At: mustTime(t, "2026-04-26T11:00:00Z"), Body: "second observation\n\nwith body"},
		{At: mustTime(t, "2026-04-27T09:30:00Z"), Body: "third"},
	}
	for _, e := range entries {
		if err := fs.AppendLog(ctx, testRepo, e); err != nil {
			t.Fatalf("AppendLog: %v", err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatalf("read log.md: %v", err)
	}
	got := string(raw)
	for _, e := range entries {
		if !strings.Contains(got, e.Body) {
			t.Errorf("log missing body %q\nfull log:\n%s", e.Body, got)
		}
		if !strings.Contains(got, e.At.UTC().Format(time.RFC3339)) {
			t.Errorf("log missing timestamp for %v", e.At)
		}
	}
}

func TestLoadOpenIssues_EmptyDir(t *testing.T) {
	fs := New(t.TempDir())
	got, err := fs.LoadOpenIssues(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("LoadOpenIssues: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestIssueLifecycle(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()
	now := mustTime(t, "2026-04-26T10:00:00Z")

	open := IssueState{
		Ref:     IssueRef{Number: 1, Path: "0001-eval-vs-assessment.md"},
		Status:  IssueOpen,
		Updated: now,
		Body:    "Hello.\n\nNoticed `eval` and `assessment` in the same file.\n\n— *Drone Honor Thy Error*",
	}
	if err := fs.RecordIssueState(ctx, testRepo, open); err != nil {
		t.Fatalf("create: %v", err)
	}

	openPath := filepath.Join(root, ".ocp", "conversation", "0001-eval-vs-assessment.md")
	if _, err := os.Stat(openPath); err != nil {
		t.Fatalf("expected open file at %s: %v", openPath, err)
	}

	refs, err := fs.LoadOpenIssues(ctx, testRepo)
	if err != nil {
		t.Fatalf("LoadOpenIssues: %v", err)
	}
	if len(refs) != 1 || refs[0].Path != "0001-eval-vs-assessment.md" || refs[0].Number != 1 {
		t.Errorf("LoadOpenIssues: unexpected refs %#v", refs)
	}

	closed := open
	closed.Status = IssueClosed
	closed.Updated = mustTime(t, "2026-04-26T11:00:00Z")
	closed.Body = open.Body + "\n\nClosed: glossary updated to canonicalize `eval`."
	if err := fs.RecordIssueState(ctx, testRepo, closed); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(openPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected open file removed after close, stat err: %v", err)
	}
	closedPath := filepath.Join(root, ".ocp", "conversation", "closed", "0001-eval-vs-assessment.md")
	if _, err := os.Stat(closedPath); err != nil {
		t.Fatalf("expected closed file at %s: %v", closedPath, err)
	}

	refs, err = fs.LoadOpenIssues(ctx, testRepo)
	if err != nil {
		t.Fatalf("LoadOpenIssues after close: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected no open issues after close, got %v", refs)
	}
}

// glossariesEqual is a deep-equality check that treats nil and empty slices
// as the same. reflect.DeepEqual treats them as different, which would make
// the round-trip test brittle for the "no synonyms" case.
func glossariesEqual(a, b Glossary) bool {
	if len(a.Terms) != len(b.Terms) {
		return false
	}
	for i := range a.Terms {
		at, bt := a.Terms[i], b.Terms[i]
		if at.Canonical != bt.Canonical || at.Definition != bt.Definition {
			return false
		}
		if len(at.Synonyms) != len(bt.Synonyms) {
			return false
		}
		if len(at.Synonyms) > 0 && !reflect.DeepEqual(at.Synonyms, bt.Synonyms) {
			return false
		}
	}
	return true
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tt
}
