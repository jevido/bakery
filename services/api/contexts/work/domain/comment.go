package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxComment is the longest body of a Comment, in characters.
const MaxComment = 20000

var (
	// ErrNotAuthor is a Comment changed by someone other than its author.
	ErrNotAuthor = errors.New("only the author of a comment may change it")
	// ErrCommentDeleted is a Comment changed after it was deleted.
	ErrCommentDeleted = errors.New("comment was deleted")
)

// Comment is a Member's Markdown message on one Issue. A deleted Comment
// keeps its place in the thread without its body. AuthorID is 0 once the
// author's account is gone, and then nobody can change it.
type Comment struct {
	ID        uint64
	IssueID   uint64
	AuthorID  uint64
	Body      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func commentBody(body string) (string, error) {
	b := strings.TrimSpace(body)
	if b == "" {
		return "", invalid("body", "body is required")
	}
	if utf8.RuneCountInString(b) > MaxComment {
		return "", invalid("body", "body is at most %d characters", MaxComment)
	}
	return b, nil
}

// NewComment is a Comment by a Member on an Issue.
func NewComment(issueID, authorID uint64, body string) (Comment, error) {
	b, err := commentBody(body)
	if err != nil {
		return Comment{}, err
	}
	return Comment{IssueID: issueID, AuthorID: authorID, Body: b}, nil
}

func (c *Comment) Deleted() bool { return c.DeletedAt != nil }

func (c *Comment) mayChange(by uint64) error {
	if c.Deleted() {
		return ErrCommentDeleted
	}
	if by == 0 || by != c.AuthorID {
		return ErrNotAuthor
	}
	return nil
}

// Edit replaces the body; only the author may.
func (c *Comment) Edit(by uint64, body string) error {
	if err := c.mayChange(by); err != nil {
		return err
	}
	b, err := commentBody(body)
	if err != nil {
		return err
	}
	c.Body = b
	return nil
}

// Delete takes the body away and marks the Comment deleted; only the
// author may.
func (c *Comment) Delete(by uint64, now time.Time) error {
	if err := c.mayChange(by); err != nil {
		return err
	}
	c.Body, c.DeletedAt = "", &now
	return nil
}
