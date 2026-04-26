package scout

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
