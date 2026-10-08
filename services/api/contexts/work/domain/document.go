package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxDocumentKey is the longest Document key, in characters.
	MaxDocumentKey = 64
	// MaxDocumentTitle is the longest title of an Issue document.
	MaxDocumentTitle = 200
	// MaxDocumentBody is the longest body of an Issue document, in
	// characters.
	MaxDocumentBody = 524288
	// MaxChangeSummary is the longest change summary of a Revision.
	MaxChangeSummary = 500
)

var (
	// ErrStaleRevision is a save whose Base revision is no longer the
	// newest: someone saved in between.
	ErrStaleRevision = errors.New("document was updated by someone else")
	// ErrRestoreNewest is a Restore of the Revision that is already the
	// newest.
	ErrRestoreNewest = errors.New("selected revision is already the latest revision")
)

// ParseDocumentKey checks a Document key: 1–64 lowercase letters, digits,
// `_` and `-`, starting with a letter or digit.
func ParseDocumentKey(key string) (string, error) {
	if key == "" || len(key) > MaxDocumentKey {
		return "", invalid("key", "key is 1 to %d characters", MaxDocumentKey)
	}
	for i, r := range key {
		letterOrDigit := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if !letterOrDigit && (i == 0 || r != '_' && r != '-') {
			return "", invalid("key", "key is lowercase letters, digits, _ and -, starting with a letter or digit")
		}
	}
	return key, nil
}

// Revision is one saved version of an Issue document. AuthorID is 0 once
// the author's account is gone.
type Revision struct {
	ID         uint64
	DocumentID uint64
	Number     int
	Title      string
	Body       string
	Summary    string
	AuthorID   uint64
	CreatedAt  time.Time
}

// IssueDocument is a titled Markdown write-up on one Issue under a
// Document key. Its title and body are those of its newest Revision,
// numbered Latest.
type IssueDocument struct {
	ID      uint64
	GuildID uint64
	IssueID uint64
	Key     string
	Title   string
	Body    string
	Latest  int
	// LatestRevisionID is the id of the newest Revision, set by the store.
	LatestRevisionID uint64
	CreatedByID      uint64
	UpdatedByID      uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func documentText(title, body, summary string) (string, string, string, error) {
	t := strings.TrimSpace(title)
	if utf8.RuneCountInString(t) > MaxDocumentTitle {
		return "", "", "", invalid("title", "title is at most %d characters", MaxDocumentTitle)
	}
	if utf8.RuneCountInString(body) > MaxDocumentBody {
		return "", "", "", invalid("body", "body is at most %d characters", MaxDocumentBody)
	}
	s := strings.TrimSpace(summary)
	if utf8.RuneCountInString(s) > MaxChangeSummary {
		return "", "", "", invalid("change_summary", "change summary is at most %d characters", MaxChangeSummary)
	}
	return t, body, s, nil
}

// NewIssueDocument is a new Issue document on an Issue with its first
// Revision.
func NewIssueDocument(guildID, issueID uint64, key, title, body, summary string, authorID uint64, now time.Time) (IssueDocument, Revision, error) {
	k, err := ParseDocumentKey(key)
	if err != nil {
		return IssueDocument{}, Revision{}, err
	}
	t, b, s, err := documentText(title, body, summary)
	if err != nil {
		return IssueDocument{}, Revision{}, err
	}
	d := IssueDocument{
		GuildID: guildID, IssueID: issueID, Key: k, Title: t, Body: b, Latest: 1,
		CreatedByID: authorID, UpdatedByID: authorID, CreatedAt: now, UpdatedAt: now,
	}
	return d, Revision{Number: 1, Title: t, Body: b, Summary: s, AuthorID: authorID, CreatedAt: now}, nil
}

func (d *IssueDocument) revise(title, body, summary string, authorID uint64, now time.Time) Revision {
	d.Title, d.Body, d.Latest, d.UpdatedByID, d.UpdatedAt = title, body, d.Latest+1, authorID, now
	return Revision{DocumentID: d.ID, Number: d.Latest, Title: title, Body: body, Summary: summary, AuthorID: authorID, CreatedAt: now}
}

// Save makes title and body the newest Revision. baseRevision is the
// number of the Revision the edit started from; anything but the newest is
// ErrStaleRevision.
func (d *IssueDocument) Save(title, body, summary string, baseRevision int, authorID uint64, now time.Time) (Revision, error) {
	if baseRevision != d.Latest {
		return Revision{}, ErrStaleRevision
	}
	t, b, s, err := documentText(title, body, summary)
	if err != nil {
		return Revision{}, err
	}
	return d.revise(t, b, s, authorID, now), nil
}

// Restore saves an older Revision's title and body as the newest Revision.
func (d *IssueDocument) Restore(old Revision, authorID uint64, now time.Time) (Revision, error) {
	if old.Number >= d.Latest {
		return Revision{}, ErrRestoreNewest
	}
	return d.revise(old.Title, old.Body, fmt.Sprintf("Restored from revision %d", old.Number), authorID, now), nil
}
