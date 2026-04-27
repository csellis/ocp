package voice

import (
	"strings"
	"testing"
)

func TestFormat_Shape(t *testing.T) {
	body := Format(Body{
		Synonym:   "vocabulary",
		Canonical: "glossary",
		Files: []FileCitation{
			{File: "docs/a.md", Lines: []int{3, 7}},
			{File: "docs/b.md", Lines: []int{12}},
		},
		Card:     "Honor thy error as a hidden intention.",
		ShipName: "Drone Honor Thy Error As A Hidden Intention",
	})

	for _, want := range []string{
		"Hello.\n",
		"I noticed `vocabulary` appearing in 2 files (3 occurrences) where the glossary canonicalizes this concept as `glossary`.",
		"Citations:",
		"- docs/a.md: 3, 7",
		"- docs/b.md: 12",
		"If `vocabulary` is a distinct concept, please add it to the glossary. Otherwise the canonical is `glossary`.",
		"*Card: Honor thy error as a hidden intention.*",
		"— *Drone Honor Thy Error As A Hidden Intention*",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in body:\n%s", want, body)
		}
	}
}

func TestFormat_PluralizeSingular(t *testing.T) {
	body := Format(Body{
		Synonym:   "issue",
		Canonical: "observation",
		Files:     []FileCitation{{File: "x.go", Lines: []int{1}}},
		ShipName:  "S",
	})
	if !strings.Contains(body, "1 file (1 occurrence)") {
		t.Errorf("expected '1 file (1 occurrence)', got:\n%s", body)
	}
	if strings.Contains(body, "1 files") || strings.Contains(body, "1 occurrences") {
		t.Errorf("singular pluralization broken:\n%s", body)
	}
}

func TestFormat_OptionalCardOmitted(t *testing.T) {
	body := Format(Body{
		Synonym:   "v",
		Canonical: "g",
		Files:     []FileCitation{{File: "x.md", Lines: []int{1}}},
		ShipName:  "S",
		// no Card
	})
	if strings.Contains(body, "*Card:") {
		t.Errorf("card line should be absent when Card is empty:\n%s", body)
	}
}

func TestFormat_OptionalSignatureOmitted(t *testing.T) {
	body := Format(Body{
		Synonym:   "v",
		Canonical: "g",
		Files:     []FileCitation{{File: "x.md", Lines: []int{1}}},
		// no ShipName
	})
	if strings.Contains(body, "— *") {
		t.Errorf("signature line should be absent when ShipName is empty:\n%s", body)
	}
}
