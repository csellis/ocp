package storage

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Filesystem implements Storage by reading and writing files under the
// .ocp directory of one repo on disk. The repo path is fixed at
// construction; the RepoID parameter on the Storage methods is ignored.
type Filesystem struct {
	root string
}

// Compile-time guarantee that *Filesystem satisfies Storage.
var _ Storage = (*Filesystem)(nil)

// New constructs a Filesystem rooted at the given repo path. The path
// must already exist; .ocp/ is created lazily on first write.
func New(root string) *Filesystem {
	return &Filesystem{root: root}
}

func (fs *Filesystem) ocpDir() string          { return filepath.Join(fs.root, ".ocp") }
func (fs *Filesystem) glossaryPath() string    { return filepath.Join(fs.ocpDir(), "glossary.md") }
func (fs *Filesystem) logPath() string         { return filepath.Join(fs.ocpDir(), "log.md") }
func (fs *Filesystem) conversationDir() string { return filepath.Join(fs.ocpDir(), "conversation") }
func (fs *Filesystem) closedDir() string       { return filepath.Join(fs.conversationDir(), "closed") }

func (fs *Filesystem) LoadGlossary(_ context.Context, _ RepoID) (Glossary, error) {
	raw, err := os.ReadFile(fs.glossaryPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Glossary{}, ErrNotFound
		}
		return Glossary{}, fmt.Errorf("read glossary: %w", err)
	}
	return parseGlossary(raw), nil
}

func (fs *Filesystem) SaveGlossary(_ context.Context, _ RepoID, g Glossary) error {
	if err := os.MkdirAll(fs.ocpDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir .ocp: %w", err)
	}
	return atomicWrite(fs.glossaryPath(), g.Markdown())
}

// AppendLog uses read-modify-write rather than O_APPEND so every on-disk
// transition is the same atomic-rename. Single-user CLI; no contention.
func (fs *Filesystem) AppendLog(_ context.Context, _ RepoID, entry LogEntry) error {
	if err := os.MkdirAll(fs.ocpDir(), 0o755); err != nil {
		return fmt.Errorf("mkdir .ocp: %w", err)
	}
	existing, err := os.ReadFile(fs.logPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read log: %w", err)
	}
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 {
		for !bytes.HasSuffix(buf.Bytes(), []byte("\n\n")) {
			buf.WriteByte('\n')
		}
	}
	fmt.Fprintf(&buf, "## %s\n\n%s\n", entry.At.UTC().Format(time.RFC3339), strings.TrimRight(entry.Body, "\n"))
	return atomicWrite(fs.logPath(), buf.Bytes())
}

func (fs *Filesystem) LoadOpenIssues(_ context.Context, _ RepoID) ([]IssueRef, error) {
	entries, err := os.ReadDir(fs.conversationDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read conversation dir: %w", err)
	}
	var refs []IssueRef
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		refs = append(refs, IssueRef{
			Number: numberFromName(e.Name()),
			Path:   e.Name(),
		})
	}
	return refs, nil
}

func (fs *Filesystem) RecordIssueState(_ context.Context, _ RepoID, state IssueState) error {
	if state.Ref.Path == "" {
		return errors.New("RecordIssueState: empty Ref.Path")
	}
	var dir string
	switch state.Status {
	case IssueOpen:
		dir = fs.conversationDir()
	case IssueClosed:
		dir = fs.closedDir()
	default:
		return fmt.Errorf("RecordIssueState: unknown status %d", state.Status)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	target := filepath.Join(dir, state.Ref.Path)
	if err := atomicWrite(target, serializeObservation(state)); err != nil {
		return err
	}
	if state.Status == IssueClosed {
		old := filepath.Join(fs.conversationDir(), state.Ref.Path)
		if old != target {
			if err := os.Remove(old); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove open copy: %w", err)
			}
		}
	}
	return nil
}

// atomicWrite writes data to path via a same-directory temp file followed
// by os.Rename. POSIX rename within one filesystem is atomic; partial
// writes never become visible.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

var nameNumberRegexp = regexp.MustCompile(`^(\d+)`)

func numberFromName(name string) int {
	m := nameNumberRegexp.FindString(name)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

// --- glossary format ---

// parseGlossary reads the format produced by serializeGlossary: optional
// `# Glossary` header, then one `## term` section per term. Body lines
// continue until the next `## ` or a `Synonyms:` line; the latter is
// always the trailing element of a section.
func parseGlossary(raw []byte) Glossary {
	var g Glossary
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	var current *Term
	var defLines []string
	flush := func() {
		if current == nil {
			return
		}
		current.Definition = strings.TrimSpace(strings.Join(defLines, "\n"))
		g.Terms = append(g.Terms, *current)
		current = nil
		defLines = nil
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "# "):
			// file header; skip
		case strings.HasPrefix(line, "## "):
			flush()
			current = &Term{Canonical: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
		case current != nil && strings.HasPrefix(line, "Synonyms:"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "Synonyms:"))
			for _, p := range strings.Split(rest, ",") {
				if s := strings.TrimSpace(p); s != "" {
					current.Synonyms = append(current.Synonyms, s)
				}
			}
		case current != nil:
			defLines = append(defLines, line)
		}
	}
	flush()
	return g
}

// Markdown serializes a Glossary to its on-disk markdown form. The
// inverse of parseGlossary; round-trips losslessly for any value
// produced by parsing or by direct construction.
func (g Glossary) Markdown() []byte {
	var b strings.Builder
	b.WriteString("# Glossary\n\n")
	for _, t := range g.Terms {
		fmt.Fprintf(&b, "## %s\n\n%s\n", t.Canonical, t.Definition)
		if len(t.Synonyms) > 0 {
			fmt.Fprintf(&b, "\nSynonyms: %s\n", strings.Join(t.Synonyms, ", "))
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// --- observation format ---

func serializeObservation(s IssueState) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "Number: %d\n", s.Ref.Number)
	fmt.Fprintf(&b, "Status: %s\n", statusString(s.Status))
	fmt.Fprintf(&b, "Updated: %s\n", s.Updated.UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimRight(s.Body, "\n"))
	b.WriteByte('\n')
	return []byte(b.String())
}

func statusString(s IssueStatus) string {
	switch s {
	case IssueOpen:
		return "open"
	case IssueClosed:
		return "closed"
	default:
		return "unknown"
	}
}
