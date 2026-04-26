// Package scout implements the cheap-stage drift detector.
//
// Scout walks a working tree and finds textual occurrences of glossary
// synonyms. No LLM, no AST, no parser per language: pure regex against
// the lines of scannable files. The output feeds either the CLI directly
// (slice 2) or the expensive-stage agent in later slices.
//
// The package's job is intentionally narrow. Cost discipline lives here:
// scout returns candidates; the LLM judges them.
package scout

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/csellis/ocp/internal/storage"
)

// Hit is one occurrence of a glossary synonym in a file.
type Hit struct {
	File      string // path relative to the scan root
	Line      int    // 1-indexed
	Synonym   string // the matched text as it appears in the file
	Canonical string // the canonical the synonym should be replaced with
	Snippet   string // the trimmed line containing the match
}

// Detect walks root for glossary synonyms in scannable files. Returns
// hits in walk order (deterministic for a given filesystem). Returns
// no hits and no error if the glossary declares no synonyms.
func Detect(ctx context.Context, root string, g storage.Glossary) ([]Hit, error) {
	entries := buildEntries(g)
	if len(entries) == 0 {
		return nil, nil
	}
	var hits []Hit
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if isExcludedDir(path, root, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isScannable(d.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		fileHits, err := scanFile(path, rel, entries)
		if err != nil {
			return err
		}
		hits = append(hits, fileHits...)
		return nil
	})
	return hits, err
}

type synEntry struct {
	canonical string
	pattern   *regexp.Regexp
}

func buildEntries(g storage.Glossary) []synEntry {
	var entries []synEntry
	for _, term := range g.Terms {
		for _, syn := range term.Synonyms {
			pat := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(syn) + `\b`)
			entries = append(entries, synEntry{canonical: term.Canonical, pattern: pat})
		}
	}
	return entries
}

// isExcludedDir filters directory walk. The walk root itself is always
// scanned; everything else hidden, build-output, or vendored is skipped.
func isExcludedDir(path, root, name string) bool {
	if path == root {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "bin", "dist":
		return true
	}
	return false
}

func isScannable(name string) bool {
	switch filepath.Ext(name) {
	case ".go", ".md", ".toml":
		return true
	}
	return false
}

func scanFile(path, rel string, entries []synEntry) ([]Hit, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var hits []Hit
	for i, line := range strings.Split(string(content), "\n") {
		for _, e := range entries {
			for _, m := range e.pattern.FindAllString(line, -1) {
				hits = append(hits, Hit{
					File:      rel,
					Line:      i + 1,
					Synonym:   m,
					Canonical: e.canonical,
					Snippet:   strings.TrimSpace(line),
				})
			}
		}
	}
	return hits, nil
}
