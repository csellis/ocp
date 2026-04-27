package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/csellis/ocp/internal/storage"
)

var fixedNow = time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

func TestSeedGlossary(t *testing.T) {
	g := seedGlossary()
	if len(g.Terms) == 0 {
		t.Fatal("seed glossary is empty")
	}
	seen := make(map[string]bool)
	for i, term := range g.Terms {
		if term.Canonical == "" {
			t.Errorf("term %d has empty Canonical", i)
		}
		if term.Definition == "" {
			t.Errorf("term %q has empty Definition", term.Canonical)
		}
		if seen[term.Canonical] {
			t.Errorf("duplicate canonical %q", term.Canonical)
		}
		seen[term.Canonical] = true
	}
}

// TestSeedRoundTrip verifies the seed survives a serialize/parse cycle.
// Catches drift between the seed shape and the on-disk format.
func TestSeedRoundTrip(t *testing.T) {
	g := seedGlossary()
	root := t.TempDir()
	fs := storage.New(root)
	ctx := context.Background()
	if err := fs.SaveGlossary(ctx, storage.RepoID(""), g); err != nil {
		t.Fatalf("SaveGlossary: %v", err)
	}
	got, err := fs.LoadGlossary(ctx, storage.RepoID(""))
	if err != nil {
		t.Fatalf("LoadGlossary: %v", err)
	}
	if len(got.Terms) != len(g.Terms) {
		t.Fatalf("term count: want %d got %d", len(g.Terms), len(got.Terms))
	}
	for i := range g.Terms {
		if got.Terms[i].Canonical != g.Terms[i].Canonical {
			t.Errorf("term %d canonical: want %q got %q", i, g.Terms[i].Canonical, got.Terms[i].Canonical)
		}
		if got.Terms[i].Definition != g.Terms[i].Definition {
			t.Errorf("term %d definition mismatch", i)
		}
	}
}

func TestRunScan_FirstRunSeeds(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := runScan(context.Background(), &buf, root, fixedNow); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ocp", "glossary.md")); err != nil {
		t.Fatalf("expected glossary.md to be created: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "wrote new glossary") {
		t.Errorf("expected 'wrote new glossary' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "# Glossary") {
		t.Errorf("expected glossary markdown in output, got:\n%s", out)
	}
}

func TestRunScan_SecondRunLoads(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("first runScan: %v", err)
	}
	var buf bytes.Buffer
	if err := runScan(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("second runScan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "loaded glossary") {
		t.Errorf("expected 'loaded glossary' on second run, got:\n%s", out)
	}
	if strings.Contains(out, "wrote new glossary") {
		t.Errorf("did not expect 'wrote new glossary' on second run, got:\n%s", out)
	}
}

func TestRunDrift_NoGlossary(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := runDrift(context.Background(), &buf, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	if !strings.Contains(buf.String(), "no glossary") {
		t.Errorf("expected hint about missing glossary, got:\n%s", buf.String())
	}
}

func TestRunDrift_NoHits(t *testing.T) {
	root := t.TempDir()
	if err := runScan(context.Background(), &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var buf bytes.Buffer
	if err := runDrift(context.Background(), &buf, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	if !strings.Contains(buf.String(), "no drift detected") {
		t.Errorf("expected 'no drift detected', got:\n%s", buf.String())
	}
}

func TestRunDrift_FilesObservations(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Two synonyms, both of canonical "glossary": one observation each,
	// even though the synonym appears in multiple files.
	writeTestFile(t, root, "docs/a.md", "the team's vocabulary matters.\n")
	writeTestFile(t, root, "docs/b.md", "more vocabulary here.\nplus ubiquitous language.\n")

	var buf bytes.Buffer
	if err := runDrift(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "2 candidates: 2 new (filed), 0 existing") {
		t.Errorf("expected summary line, got:\n%s", out)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatalf("read conversation dir: %v", err)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	if len(files) != 2 {
		t.Fatalf("want 2 observation files (one per synonym, NOT per file), got %d: %v", len(files), files)
	}

	// The vocabulary candidate's body should cite both files.
	var vocabFile string
	for _, f := range files {
		if strings.Contains(f, "vocabulary") {
			vocabFile = f
			break
		}
	}
	if vocabFile == "" {
		t.Fatalf("expected a vocabulary observation, got: %v", files)
	}
	content, err := os.ReadFile(filepath.Join(convDir, vocabFile))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	for _, want := range []string{
		"Status: open",
		"Hello.",
		"canonicalizes this concept as `glossary`",
		"docs/a.md",
		"docs/b.md",
		"2 files (2 occurrences)",
		"— *Drone Honor Thy Error As A Hidden Intention*",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s:\n%s", want, vocabFile, s)
		}
	}
}

func TestRunDrift_Idempotent(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "docs/thesis.md", "the team's vocabulary matters.\n")

	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("first runDrift: %v", err)
	}

	var buf bytes.Buffer
	if err := runDrift(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("second runDrift: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "1 candidate: 0 new (filed), 1 existing") {
		t.Errorf("expected idempotent summary, got:\n%s", out)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatalf("read conversation dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 observation file after second run, got %d", count)
	}
}

func TestRunDrift_NumberingPicksUpFromExisting(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Pre-stage an existing observation at number 0042 with a different slug.
	convDir := filepath.Join(root, ".ocp", "conversation")
	if err := os.MkdirAll(convDir, 0o755); err != nil {
		t.Fatal(err)
	}
	preExisting := filepath.Join(convDir, "0042-prior-existing.md")
	if err := os.WriteFile(preExisting, []byte("---\nNumber: 42\nStatus: open\nUpdated: 2026-04-25T00:00:00Z\n---\n\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "docs/thesis.md", "the team's vocabulary matters.\n")

	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}

	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatal(err)
	}
	gotNumbers := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			gotNumbers = append(gotNumbers, e.Name())
		}
	}
	wantPrefix := "0043-"
	found := false
	for _, n := range gotNumbers {
		if strings.HasPrefix(n, wantPrefix) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected new file with prefix %s; got %v", wantPrefix, gotNumbers)
	}
}

func TestRunDrift_DedupesPerLine(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Two occurrences of "vocabulary" on line 1: should dedupe to one citation.
	writeTestFile(t, root, "doc.md", "vocabulary again vocabulary\n")

	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 observation, got %d", len(entries))
	}
	raw, err := os.ReadFile(filepath.Join(convDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	// Body should report 1 occurrence (deduped from 2 matches), and citation
	// line should be exactly "- doc.md: 1".
	s := string(raw)
	if !strings.Contains(s, "1 file (1 occurrence)") {
		t.Errorf("expected '1 file (1 occurrence)' in body, got:\n%s", s)
	}
	if !strings.Contains(s, "- doc.md: 1\n") {
		t.Errorf("expected '- doc.md: 1' citation line, got:\n%s", s)
	}
}

func TestRunScan_FirstRunWritesLog(t *testing.T) {
	root := t.TempDir()
	if err := runScan(context.Background(), &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	logBytes, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatalf("expected log.md to exist: %v", err)
	}
	if !strings.Contains(string(logBytes), "scan seeded glossary") {
		t.Errorf("missing seed entry in log.md:\n%s", logBytes)
	}
}

func TestRunScan_SecondRunDoesNotLog(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("first runScan: %v", err)
	}
	beforeLog, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	later := fixedNow.Add(1 * time.Hour)
	if err := runScan(ctx, &bytes.Buffer{}, root, later); err != nil {
		t.Fatalf("second runScan: %v", err)
	}
	afterLog, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeLog) != string(afterLog) {
		t.Errorf("second scan should not change log.md\nbefore:\n%s\nafter:\n%s", beforeLog, afterLog)
	}
}

func TestRunDrift_FilingWritesLog(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	logBytes, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatalf("expected log.md to exist: %v", err)
	}
	s := string(logBytes)
	if !strings.Contains(s, "drift filed 1 observation") {
		t.Errorf("missing drift entry in log.md:\n%s", s)
	}
	if !strings.Contains(s, "vocabulary-glossary.md") {
		t.Errorf("missing observation filename in log.md:\n%s", s)
	}
}

func TestRunDrift_NoOpDoesNotLog(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("first drift: %v", err)
	}
	beforeLog, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	later := fixedNow.Add(1 * time.Hour)
	if err := runDrift(ctx, &bytes.Buffer{}, root, later); err != nil {
		t.Fatalf("second drift: %v", err)
	}
	afterLog, err := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeLog) != string(afterLog) {
		t.Errorf("idempotent drift should not change log.md\nbefore:\n%s\nafter:\n%s", beforeLog, afterLog)
	}
}

func TestRunDrift_ZeroHitsDoesNotLog(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// No code files; scan logged its seed entry but drift has nothing to find.
	beforeLog, _ := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	afterLog, _ := os.ReadFile(filepath.Join(root, ".ocp", "log.md"))
	if string(beforeLog) != string(afterLog) {
		t.Errorf("zero-hit drift should not change log.md\nbefore:\n%s\nafter:\n%s", beforeLog, afterLog)
	}
}

func TestParseReply(t *testing.T) {
	cases := []struct {
		name string
		body string
		want replyAction
	}{
		{
			name: "no reply section",
			body: "Hello.\n\nObservation body.\n",
			want: replyAction{kind: replyNone},
		},
		{
			name: "empty reply section",
			body: "Hello.\n\n## Reply\n\n",
			want: replyAction{kind: replyNone},
		},
		{
			name: "close action",
			body: "Hello.\n\n## Reply\n\nclose: pedagogical, not drift\n",
			want: replyAction{kind: replyClose, value: "pedagogical, not drift"},
		},
		{
			name: "synonym action",
			body: "Hello.\n\n## Reply\n\nsynonym: vocab\n",
			want: replyAction{kind: replySynonym, value: "vocab"},
		},
		{
			name: "stand by",
			body: "Hello.\n\n## Reply\n\nstand by, will resolve in next sprint\n",
			want: replyAction{kind: replyStandBy},
		},
		{
			name: "unrecognized reply",
			body: "Hello.\n\n## Reply\n\nmaybe later?\n",
			want: replyAction{kind: replyNone},
		},
		{
			name: "case-insensitive heading + keyword",
			body: "Hello.\n\n## REPLY\n\nCLOSE: shouty closure\n",
			want: replyAction{kind: replyClose, value: "shouty closure"},
		},
		{
			name: "reply followed by another section",
			body: "Hello.\n\n## Reply\n\nclose: yes\n\n## Notes\n\nmore stuff\n",
			want: replyAction{kind: replyClose, value: "yes"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseReply(tc.body)
			if got.kind != tc.want.kind || got.value != tc.want.value {
				t.Errorf("parseReply: want %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestRunRespond_NoOpenIssues(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var buf bytes.Buffer
	if err := runRespond(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(buf.String(), "no open observations") {
		t.Errorf("expected 'no open observations', got:\n%s", buf.String())
	}
}

func TestRunRespond_NoGlossary(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := runRespond(context.Background(), &buf, root, fixedNow); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(buf.String(), "no glossary") {
		t.Errorf("expected 'no glossary' hint, got:\n%s", buf.String())
	}
}

func TestRunRespond_CloseAction(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatal(err)
	}
	var obs string
	for _, e := range entries {
		if !e.IsDir() {
			obs = e.Name()
			break
		}
	}
	if obs == "" {
		t.Fatal("no observation filed")
	}
	obsPath := filepath.Join(convDir, obs)
	raw, _ := os.ReadFile(obsPath)
	withReply := string(raw) + "\n## Reply\n\nclose: pedagogical use\n"
	if err := os.WriteFile(obsPath, []byte(withReply), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	later := fixedNow.Add(1 * time.Hour)
	if err := runRespond(ctx, &buf, root, later); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(buf.String(), "1 closed") {
		t.Errorf("expected '1 closed', got:\n%s", buf.String())
	}

	if _, err := os.Stat(obsPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("open observation should be gone after close, stat err: %v", err)
	}
	closedPath := filepath.Join(convDir, "closed", obs)
	closedBytes, err := os.ReadFile(closedPath)
	if err != nil {
		t.Fatalf("expected closed observation: %v", err)
	}
	if !strings.Contains(string(closedBytes), "Closed: pedagogical use") {
		t.Errorf("missing closure note in:\n%s", closedBytes)
	}
}

func TestRunRespond_SynonymAction(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "team has its own vocab.\n")
	// Add "vocab" as a synonym of glossary so the drift run files an observation.
	gloss, _ := storage.New(root).LoadGlossary(ctx, storage.RepoID(""))
	for i := range gloss.Terms {
		if gloss.Terms[i].Canonical == "glossary" {
			gloss.Terms[i].Synonyms = append(gloss.Terms[i].Synonyms, "vocab")
		}
	}
	if err := storage.New(root).SaveGlossary(ctx, storage.RepoID(""), gloss); err != nil {
		t.Fatal(err)
	}
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}

	convDir := filepath.Join(root, ".ocp", "conversation")
	entries, _ := os.ReadDir(convDir)
	var obsPath string
	for _, e := range entries {
		if !e.IsDir() {
			obsPath = filepath.Join(convDir, e.Name())
			break
		}
	}
	raw, _ := os.ReadFile(obsPath)
	withReply := string(raw) + "\n## Reply\n\nsynonym: vocab\n"
	if err := os.WriteFile(obsPath, []byte(withReply), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runRespond(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "1 closed") {
		t.Errorf("expected '1 closed', got:\n%s", out)
	}
	// Glossary update count: vocab was already a synonym (we added it above), so 0.
	if !strings.Contains(out, "0 glossary updates") {
		t.Errorf("expected '0 glossary updates' (vocab already present), got:\n%s", out)
	}
}

func TestRunRespond_NoReplySkips(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("seed: %v", err)
	}
	writeTestFile(t, root, "doc.md", "the team's vocabulary matters.\n")
	if err := runDrift(ctx, &bytes.Buffer{}, root, fixedNow); err != nil {
		t.Fatalf("drift: %v", err)
	}

	var buf bytes.Buffer
	if err := runRespond(ctx, &buf, root, fixedNow); err != nil {
		t.Fatalf("runRespond: %v", err)
	}
	if !strings.Contains(buf.String(), "1 skipped") {
		t.Errorf("expected '1 skipped' for observation with no Reply, got:\n%s", buf.String())
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"vocabulary", "vocabulary"},
		{"Vocabulary", "vocabulary"},
		{"ubiquitous language", "ubiquitous-language"},
		{"docs/THESIS.md", "docs-thesis-md"},
		{"  multiple   spaces ", "multiple-spaces"},
		{"already-slugged", "already-slugged"},
		{"!!!leading-junk", "leading-junk"},
	}
	for _, tc := range cases {
		if got := slugify(tc.in); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugFromPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0001-vocabulary-docs.md", "vocabulary-docs"},
		{"0042-foo.md", "foo"},
		{"no-prefix.md", "no-prefix"},
	}
	for _, tc := range cases {
		if got := slugFromPath(tc.in); got != tc.want {
			t.Errorf("slugFromPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
