// Package voice formats observation bodies in the OCP voice.
//
// Format takes a fully populated Body value and returns the markdown.
// All inputs are explicit: Format does no I/O, picks no random, makes
// no time calls. Callers pick the Oblique card (PickCard) and the
// ship-name (internal/names) and pass them in. This keeps Format pure
// and tests deterministic.
package voice

import (
	"fmt"
	"strings"
)

// Body is the fully resolved input to Format.
type Body struct {
	Synonym   string
	Canonical string
	Files     []FileCitation
	Card      string // optional Oblique card; empty means no card line
	ShipName  string // signature; empty means no signature line
}

// FileCitation is the per-file evidence for one observation.
type FileCitation struct {
	File  string
	Lines []int
}

// Format returns the markdown body for one observation in OCP voice.
// Shape:
//
//	Hello.
//
//	I noticed `<syn>` appearing in N file(s) (M occurrence(s)) where
//	the glossary canonicalizes this concept as `<canonical>`.
//
//	Citations:
//	- file: line, line, ...
//
//	If `<syn>` is a distinct concept, please add it to the glossary.
//	Otherwise the canonical is `<canonical>`.
//
//	*Card: <card>.*
//
//	— *<ship-name>*
func Format(b Body) string {
	var s strings.Builder
	s.WriteString("Hello.\n\n")

	totalLines := 0
	for _, f := range b.Files {
		totalLines += len(f.Lines)
	}
	fmt.Fprintf(&s, "I noticed `%s` appearing in %d %s (%d %s) where the glossary canonicalizes this concept as `%s`.\n\n",
		b.Synonym,
		len(b.Files), pluralize("file", len(b.Files)),
		totalLines, pluralize("occurrence", totalLines),
		b.Canonical)

	s.WriteString("Citations:\n")
	for _, f := range b.Files {
		fmt.Fprintf(&s, "- %s:", f.File)
		for i, ln := range f.Lines {
			if i == 0 {
				fmt.Fprintf(&s, " %d", ln)
			} else {
				fmt.Fprintf(&s, ", %d", ln)
			}
		}
		s.WriteByte('\n')
	}
	s.WriteByte('\n')

	fmt.Fprintf(&s, "If `%s` is a distinct concept, please add it to the glossary. Otherwise the canonical is `%s`.\n",
		b.Synonym, b.Canonical)

	if b.Card != "" {
		fmt.Fprintf(&s, "\n*Card: %s*\n", b.Card)
	}
	if b.ShipName != "" {
		fmt.Fprintf(&s, "\n— *%s*\n", b.ShipName)
	}
	return s.String()
}

func pluralize(s string, n int) string {
	if n == 1 {
		return s
	}
	return s + "s"
}
