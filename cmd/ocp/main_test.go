package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csellis/ocp/internal/storage"
)

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
	if err := runScan(context.Background(), &buf, root); err != nil {
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
	if err := runScan(ctx, &bytes.Buffer{}, root); err != nil {
		t.Fatalf("first runScan: %v", err)
	}
	var buf bytes.Buffer
	if err := runScan(ctx, &buf, root); err != nil {
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
	if err := runDrift(context.Background(), &buf, root); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	if !strings.Contains(buf.String(), "no glossary") {
		t.Errorf("expected hint about missing glossary, got:\n%s", buf.String())
	}
}

func TestRunDrift_NoHits(t *testing.T) {
	root := t.TempDir()
	if err := runScan(context.Background(), &bytes.Buffer{}, root); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Working tree has no .md/.go/.toml files, so even with synonyms in the seed
	// glossary there is nothing to scan.
	var buf bytes.Buffer
	if err := runDrift(context.Background(), &buf, root); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	if !strings.Contains(buf.String(), "no drift detected") {
		t.Errorf("expected 'no drift detected', got:\n%s", buf.String())
	}
}

func TestRunDrift_FindsSynonyms(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	if err := runScan(ctx, &bytes.Buffer{}, root); err != nil {
		t.Fatalf("seed: %v", err)
	}
	doc := filepath.Join(root, "docs", "thesis.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("the team's vocabulary matters.\nthe ubiquitous language too.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runDrift(ctx, &buf, root); err != nil {
		t.Fatalf("runDrift: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"vocabulary", "ubiquitous language", "canonical: glossary", "2 hits"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
