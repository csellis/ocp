package scout

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/csellis/ocp/internal/storage"
)

func TestDetect_NoSynonymsInGlossary(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "x.md", "the team's vocabulary matters.")
	g := storage.Glossary{Terms: []storage.Term{{Canonical: "glossary", Definition: "..."}}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want zero hits when glossary has no synonyms, got %d", len(hits))
	}
}

func TestDetect_FindsSynonyms(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/thesis.md", "first line\nthe team's vocabulary matters.\nthird line\n")
	writeFile(t, root, "src/main.go", "// Vocabulary is part of the team.\n")
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d: %#v", len(hits), hits)
	}
	sortHits(hits)
	want := []Hit{
		{File: "docs/thesis.md", Line: 2, Synonym: "vocabulary", Canonical: "glossary"},
		{File: "src/main.go", Line: 1, Synonym: "Vocabulary", Canonical: "glossary"},
	}
	for i := range want {
		if hits[i].File != want[i].File || hits[i].Line != want[i].Line ||
			hits[i].Synonym != want[i].Synonym || hits[i].Canonical != want[i].Canonical {
			t.Errorf("hit %d: want %+v, got %+v", i, want[i], hits[i])
		}
	}
}

func TestDetect_MultiWordSynonym(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "thesis.md", "the team's ubiquitous language matters.")
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"ubiquitous language"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d: %#v", len(hits), hits)
	}
	if hits[0].Synonym != "ubiquitous language" {
		t.Errorf("want synonym 'ubiquitous language', got %q", hits[0].Synonym)
	}
}

func TestDetect_WordBoundary(t *testing.T) {
	// "obs" must not match inside "observation". Word boundaries enforce this.
	root := t.TempDir()
	writeFile(t, root, "x.md", "the observation matters.")
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "observation", Synonyms: []string{"obs"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want zero hits (substring should not match), got %d: %#v", len(hits), hits)
	}
}

func TestDetect_SkipsHiddenAndExcludedDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "x.md", "vocabulary")                // included
	writeFile(t, root, ".ocp/glossary.md", "vocabulary")    // skipped: hidden
	writeFile(t, root, ".git/HEAD", "vocabulary")           // skipped: hidden
	writeFile(t, root, "node_modules/foo.md", "vocabulary") // skipped: excluded
	writeFile(t, root, "vendor/bar.md", "vocabulary")       // skipped: excluded
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit (only x.md), got %d: %#v", len(hits), hits)
	}
	if hits[0].File != "x.md" {
		t.Errorf("want file 'x.md', got %q", hits[0].File)
	}
}

func TestDetect_OnlyScannableExtensions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "x.md", "vocabulary")   // included
	writeFile(t, root, "x.go", "vocabulary")   // included
	writeFile(t, root, "x.toml", "vocabulary") // included
	writeFile(t, root, "x.txt", "vocabulary")  // skipped: not in allowlist
	writeFile(t, root, "x.json", "vocabulary") // skipped
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(hits) != 3 {
		t.Errorf("want 3 hits (md, go, toml), got %d: %#v", len(hits), hits)
	}
}

func TestDetect_ContextCancellation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "x.md", "vocabulary")
	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Detect(ctx, root, g)
	if err == nil {
		t.Fatal("want error from cancelled context, got nil")
	}
}

func TestDetect_GitignoreRespected(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")

	writeFile(t, root, ".gitignore", "ignored.md\n")
	writeFile(t, root, "tracked.md", "the team's vocabulary matters.\n")
	writeFile(t, root, "ignored.md", "more vocabulary here.\n")
	writeFile(t, root, "untracked.md", "vocabulary in an untracked but not ignored file.\n")
	runGit(t, root, "add", ".gitignore", "tracked.md")
	runGit(t, root, "commit", "-q", "-m", "init")

	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	gotFiles := map[string]bool{}
	for _, h := range hits {
		gotFiles[h.File] = true
	}
	if !gotFiles["tracked.md"] {
		t.Errorf("expected tracked.md in hits, got %v", gotFiles)
	}
	if !gotFiles["untracked.md"] {
		t.Errorf("expected untracked.md in hits (not gitignored), got %v", gotFiles)
	}
	if gotFiles["ignored.md"] {
		t.Errorf("ignored.md should be skipped via .gitignore, got %v", gotFiles)
	}
}

func TestDetect_TestdataAlwaysExcluded(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, root, "tracked.md", "vocabulary here.\n")
	writeFile(t, root, "fixtures/testdata/sample.md", "vocabulary here too.\n")
	writeFile(t, root, "testdata/top.md", "and vocabulary at the top.\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")

	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	files := map[string]bool{}
	for _, h := range hits {
		files[h.File] = true
	}
	if !files["tracked.md"] {
		t.Errorf("expected tracked.md in hits, got %v", files)
	}
	for f := range files {
		if strings.Contains(f, "testdata") {
			t.Errorf("path with testdata component must be excluded, got %q", f)
		}
	}
}

func TestDetect_DotOcpAlwaysExcluded(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	// Track .ocp/glossary.md explicitly (no .gitignore on it). Scout must
	// still skip it: .ocp is OCP's own state.
	writeFile(t, root, ".ocp/glossary.md", "## glossary\n\nSynonyms: vocabulary\n")
	writeFile(t, root, "tracked.md", "vocabulary here.\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")

	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for _, h := range hits {
		if h.File == ".ocp/glossary.md" {
			t.Errorf(".ocp/glossary.md must always be excluded, got hit %#v", h)
		}
	}
}

func TestDetect_TrackedButDeletedSkippedSilently(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writeFile(t, root, "kept.md", "vocabulary stays.\n")
	writeFile(t, root, "doomed.md", "vocabulary about to vanish.\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")

	// Remove from disk but leave in the index. `git ls-files --cached`
	// will still list doomed.md; scout must not error on the missing read.
	if err := os.Remove(filepath.Join(root, "doomed.md")); err != nil {
		t.Fatal(err)
	}

	g := storage.Glossary{Terms: []storage.Term{
		{Canonical: "glossary", Synonyms: []string{"vocabulary"}},
	}}
	hits, err := Detect(context.Background(), root, g)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	files := map[string]bool{}
	for _, h := range hits {
		files[h.File] = true
	}
	if !files["kept.md"] {
		t.Errorf("expected kept.md in hits, got %v", files)
	}
	if files["doomed.md"] {
		t.Errorf("doomed.md was deleted from disk; expected silent skip, got %v", files)
	}
}

// runGit runs a git command in dir with deterministic author/committer
// env so tests do not depend on the user's git config.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=ocp-test",
		"GIT_AUTHOR_EMAIL=ocp-test@example.com",
		"GIT_COMMITTER_NAME=ocp-test",
		"GIT_COMMITTER_EMAIL=ocp-test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func sortHits(hits []Hit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
}
