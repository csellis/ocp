// Command eval runs OCP's scout against a labeled corpus and reports
// precision and recall on the synonymy task. Invoke from the module
// root: `go run ./eval` or `make eval`.
//
// Each subdirectory of eval/corpus/ is one example: testdata/ holds
// the fixture filesystem (named so Go's tooling ignores any .go files
// inside), glossary.md is the glossary scout sees, and expected.json
// lists the expected hits.
//
// Hits are compared as (file, line, synonym, canonical) 4-tuples;
// synonym match is case-insensitive so corpus authors do not have to
// worry about how scout reports the matched text.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/csellis/ocp/internal/scout"
	"github.com/csellis/ocp/internal/storage"
)

const corpusDir = "eval/corpus"

type labeledHit struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Synonym   string `json:"synonym"`
	Canonical string `json:"canonical"`
}

type expectedFile struct {
	Hits []labeledHit `json:"hits"`
}

type result struct {
	Name       string
	TP, FP, FN int
}

func (r result) precision() float64 {
	if r.TP+r.FP == 0 {
		return 1.0
	}
	return float64(r.TP) / float64(r.TP+r.FP)
}

func (r result) recall() float64 {
	if r.TP+r.FN == 0 {
		return 1.0
	}
	return float64(r.TP) / float64(r.TP+r.FN)
}

func main() {
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		die("read corpus: %v", err)
	}

	var results []result
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r, err := evalOne(filepath.Join(corpusDir, e.Name()), e.Name())
		if err != nil {
			die("%s: %v", e.Name(), err)
		}
		results = append(results, r)
	}
	printReport(results)
}

func evalOne(dir, name string) (result, error) {
	glossaryBytes, err := os.ReadFile(filepath.Join(dir, "glossary.md"))
	if err != nil {
		return result{}, fmt.Errorf("read glossary: %w", err)
	}
	g := storage.ParseGlossary(glossaryBytes)

	expectedBytes, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		return result{}, fmt.Errorf("read expected: %w", err)
	}
	var exp expectedFile
	if err := json.Unmarshal(expectedBytes, &exp); err != nil {
		return result{}, fmt.Errorf("parse expected: %w", err)
	}

	hits, err := scout.Detect(context.Background(), filepath.Join(dir, "testdata"), g)
	if err != nil {
		return result{}, fmt.Errorf("scout: %w", err)
	}

	expectedSet := makeHitSet(exp.Hits)
	actualSet := makeHitSet(hitsFromScout(hits))

	r := result{Name: name}
	for k := range actualSet {
		if expectedSet[k] {
			r.TP++
		} else {
			r.FP++
		}
	}
	for k := range expectedSet {
		if !actualSet[k] {
			r.FN++
		}
	}
	return r, nil
}

func hitsFromScout(hits []scout.Hit) []labeledHit {
	out := make([]labeledHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, labeledHit{
			File:      h.File,
			Line:      h.Line,
			Synonym:   h.Synonym,
			Canonical: h.Canonical,
		})
	}
	return out
}

// makeHitSet normalizes synonym casing so corpus authors do not have to
// match scout's case-preservation behavior exactly.
func makeHitSet(hits []labeledHit) map[labeledHit]bool {
	out := make(map[labeledHit]bool, len(hits))
	for _, h := range hits {
		h.Synonym = strings.ToLower(h.Synonym)
		out[h] = true
	}
	return out
}

func printReport(results []result) {
	fmt.Println("# OCP Eval Report")
	fmt.Println()
	fmt.Println("| Example | TP | FP | FN | Precision | Recall |")
	fmt.Println("|---|---|---|---|---|---|")
	var totalTP, totalFP, totalFN int
	for _, r := range results {
		fmt.Printf("| %s | %d | %d | %d | %.3f | %.3f |\n",
			r.Name, r.TP, r.FP, r.FN, r.precision(), r.recall())
		totalTP += r.TP
		totalFP += r.FP
		totalFN += r.FN
	}
	agg := result{Name: "AGGREGATE", TP: totalTP, FP: totalFP, FN: totalFN}
	fmt.Printf("| **%s** | **%d** | **%d** | **%d** | **%.3f** | **%.3f** |\n",
		agg.Name, agg.TP, agg.FP, agg.FN, agg.precision(), agg.recall())
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "eval: "+format+"\n", args...)
	os.Exit(1)
}
