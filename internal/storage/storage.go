// Package storage holds OCP's persistent state for one repo.
//
// The Storage interface has two implementations: Filesystem (v0.1, this
// package) writes to .ocp/ inside the watched repo; Firestore (v0.2) writes
// to GCP. The interface is the seam; the agent never knows which backend
// is wired up.
package storage

import (
	"context"
	"errors"
	"time"
)

// RepoID identifies a repo across implementations. For Firestore it is the
// document key (owner/name). For Filesystem it is informational only; the
// instance is constructed with a path on disk.
type RepoID string

// Storage is the persistence seam. All methods take context as the first
// parameter and a RepoID as the second, even where one implementation does
// not use the RepoID, so the interface is uniform across backends.
type Storage interface {
	LoadGlossary(ctx context.Context, repo RepoID) (Glossary, error)
	SaveGlossary(ctx context.Context, repo RepoID, g Glossary) error
	AppendLog(ctx context.Context, repo RepoID, entry LogEntry) error
	LoadOpenIssues(ctx context.Context, repo RepoID) ([]IssueRef, error)
	AllIssueRefs(ctx context.Context, repo RepoID) ([]IssueRef, error)
	RecordIssueState(ctx context.Context, repo RepoID, state IssueState) error
}

// ErrNotFound is returned by Load* methods when the underlying file or
// document does not exist. Callers should treat this as "no state yet,"
// not as a failure: a fresh repo has no glossary until the first write.
var ErrNotFound = errors.New("storage: not found")

// Glossary is the team's ubiquitous language as OCP holds it in memory.
// Order is preserved on round-trip so humans editing the file see no
// gratuitous reordering when OCP rewrites it.
type Glossary struct {
	Terms []Term
}

// Term is one canonical concept plus its definition and any recorded
// synonyms (which OCP would discourage from drifting back into the code).
type Term struct {
	Canonical  string
	Definition string
	Synonyms   []string
}

// LogEntry is one entry appended to .ocp/log.md. The body is freeform
// markdown; OCP writes its own observations here when running on itself.
type LogEntry struct {
	At   time.Time
	Body string
}

// IssueRef points to one observation. In Mode A it is a path under
// .ocp/conversation/; in Mode B it is a GitHub issue number. Both fields
// may be set; consumers use whichever the active backend populates.
type IssueRef struct {
	Number int    // GitHub issue number (Mode B)
	Path   string // filesystem path (Mode A)
}

// IssueStatus is the lifecycle state of an observation.
type IssueStatus int

const (
	IssueOpen IssueStatus = iota
	IssueClosed
)

// IssueState is what the conversation loop hands back to storage when an
// observation transitions. The Body is the closing comment when status is
// IssueClosed; empty otherwise.
type IssueState struct {
	Ref     IssueRef
	Status  IssueStatus
	Updated time.Time
	Body    string
}
