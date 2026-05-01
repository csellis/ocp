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

func TestAllIssueRefs(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()
	now := mustTime(t, "2026-04-26T10:00:00Z")

	// File two: one open, one closed.
	open := IssueState{
		Ref:          IssueRef{Number: 1, Path: "0001-open.md"},
		Status:       IssueOpen,
		FirstSeen:    now,
		LastReviewed: now,
		Body:         "open obs",
	}
	closed := IssueState{
		Ref:          IssueRef{Number: 2, Path: "0002-closed.md"},
		Status:       IssueClosed,
		FirstSeen:    now,
		LastReviewed: now,
		Body:         "closed obs",
	}
	if err := fs.RecordIssueState(ctx, testRepo, open); err != nil {
		t.Fatalf("record open: %v", err)
	}
	if err := fs.RecordIssueState(ctx, testRepo, closed); err != nil {
		t.Fatalf("record closed: %v", err)
	}

	refs, err := fs.AllIssueRefs(ctx, testRepo)
	if err != nil {
		t.Fatalf("AllIssueRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d: %#v", len(refs), refs)
	}
	got := map[string]bool{}
	for _, r := range refs {
		got[r.Path] = true
	}
	for _, want := range []string{"0001-open.md", "0002-closed.md"} {
		if !got[want] {
			t.Errorf("missing %s in refs %#v", want, refs)
		}
	}
}

func TestAllIssueRefs_EmptyRepo(t *testing.T) {
	fs := New(t.TempDir())
	refs, err := fs.AllIssueRefs(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("AllIssueRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("want empty, got %v", refs)
	}
}

func TestLoadIssue_RoundTrip(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()
	now := mustTime(t, "2026-04-26T10:00:00Z")

	want := IssueState{
		Ref:          IssueRef{Number: 7, Path: "0007-vocab-glossary.md"},
		Status:       IssueOpen,
		Term:         "vocabulary",
		Canonical:    "glossary",
		Files:        4,
		Occurrences:  47,
		FirstSeen:    now,
		LastReviewed: now,
		Body:         "# vocabulary -> glossary\n\nUsed in 4 files (47 occurrences):\n\n- doc.md: 1, 2, 3\n",
	}
	if err := fs.RecordIssueState(ctx, testRepo, want); err != nil {
		t.Fatalf("record: %v", err)
	}

	got, err := fs.LoadIssue(ctx, testRepo, want.Ref)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Ref != want.Ref {
		t.Errorf("Ref: want %#v got %#v", want.Ref, got.Ref)
	}
	if got.Status != want.Status {
		t.Errorf("Status: want %v got %v", want.Status, got.Status)
	}
	if got.Term != want.Term {
		t.Errorf("Term: want %q got %q", want.Term, got.Term)
	}
	if got.Canonical != want.Canonical {
		t.Errorf("Canonical: want %q got %q", want.Canonical, got.Canonical)
	}
	if got.Files != want.Files || got.Occurrences != want.Occurrences {
		t.Errorf("Files/Occurrences: want %d/%d got %d/%d", want.Files, want.Occurrences, got.Files, got.Occurrences)
	}
	if !got.FirstSeen.Equal(want.FirstSeen) {
		t.Errorf("FirstSeen: want %v got %v", want.FirstSeen, got.FirstSeen)
	}
	if !got.LastReviewed.Equal(want.LastReviewed) {
		t.Errorf("LastReviewed: want %v got %v", want.LastReviewed, got.LastReviewed)
	}
	if !strings.Contains(got.Body, "Used in 4 files") {
		t.Errorf("Body missing content: %q", got.Body)
	}
}

func TestLoadIssue_FromClosedDir(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()
	now := mustTime(t, "2026-04-26T10:00:00Z")

	state := IssueState{
		Ref:          IssueRef{Number: 1, Path: "0001-foo.md"},
		Status:       IssueClosed,
		FirstSeen:    now,
		LastReviewed: now,
		Body:         "closed",
	}
	if err := fs.RecordIssueState(ctx, testRepo, state); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, err := fs.LoadIssue(ctx, testRepo, state.Ref)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Status != IssueClosed {
		t.Errorf("want IssueClosed, got %v", got.Status)
	}
}

func TestLoadIssue_Missing(t *testing.T) {
	fs := New(t.TempDir())
	_, err := fs.LoadIssue(context.Background(), testRepo, IssueRef{Path: "missing.md"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestIssueLifecycle(t *testing.T) {
	root := t.TempDir()
	fs := New(root)
	ctx := context.Background()
	now := mustTime(t, "2026-04-26T10:00:00Z")

	open := IssueState{
		Ref:          IssueRef{Number: 1, Path: "0001-eval-vs-assessment.md"},
		Status:       IssueOpen,
		Term:         "assessment",
		Canonical:    "eval",
		Files:        1,
		Occurrences:  1,
		FirstSeen:    now,
		LastReviewed: now,
		Body:         "# assessment -> eval\n\nUsed in 1 file (1 occurrence):\n\n- foo.go: 42\n",
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
	closed.LastReviewed = mustTime(t, "2026-04-26T11:00:00Z")
	closed.ClosedReason = "glossary updated to canonicalize `eval`"
	if err := fs.RecordIssueState(ctx, testRepo, closed); err != nil {
		t.Fatalf("close: %v", err)
	}

	// File stays in conversation/; status changes via frontmatter only.
	if _, err := os.Stat(openPath); err != nil {
		t.Errorf("expected file at %s after close, stat err: %v", openPath, err)
	}
	closedDir := filepath.Join(root, ".ocp", "conversation", "closed")
	if _, err := os.Stat(closedDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("conversation/closed/ should not be created; stat err: %v", err)
	}

	got, err := fs.LoadIssue(ctx, testRepo, closed.Ref)
	if err != nil {
		t.Fatalf("LoadIssue after close: %v", err)
	}
	if got.Status != IssueClosed {
		t.Errorf("Status: want IssueClosed, got %v", got.Status)
	}
	if got.ClosedReason != closed.ClosedReason {
		t.Errorf("ClosedReason: want %q, got %q", closed.ClosedReason, got.ClosedReason)
	}

	refs, err = fs.LoadOpenIssues(ctx, testRepo)
	if err != nil {
		t.Fatalf("LoadOpenIssues after close: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected no open refs after close, got %v", refs)
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
