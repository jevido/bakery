package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewComment(t *testing.T) {
	c, err := NewComment(1, 2, "  hello  ")
	if err != nil || c.Body != "hello" || c.IssueID != 1 || c.AuthorID != 2 {
		t.Fatalf("NewComment = %+v, %v", c, err)
	}
	for _, body := range []string{"", "   ", strings.Repeat("é", MaxComment+1)} {
		var fe *FieldError
		if _, err := NewComment(1, 2, body); !errors.As(err, &fe) || fe.Field != "body" {
			t.Errorf("NewComment(%d chars) error = %v", len(body), err)
		}
	}
	if _, err := NewComment(1, 2, strings.Repeat("é", MaxComment)); err != nil {
		t.Errorf("NewComment(max) error = %v", err)
	}
}

func TestCommentEdit(t *testing.T) {
	c, _ := NewComment(1, 2, "hello")
	if err := c.Edit(3, "hi"); !errors.Is(err, ErrNotAuthor) {
		t.Errorf("Edit by another = %v", err)
	}
	if err := c.Edit(2, " "); err == nil {
		t.Error("Edit to empty body succeeded")
	}
	if err := c.Edit(2, "hi"); err != nil || c.Body != "hi" {
		t.Errorf("Edit by author = %v, body %q", err, c.Body)
	}
	gone := Comment{IssueID: 1, Body: "x"}
	if err := gone.Edit(0, "hi"); !errors.Is(err, ErrNotAuthor) {
		t.Errorf("Edit of a Comment without author = %v", err)
	}
}

func TestCommentDelete(t *testing.T) {
	c, _ := NewComment(1, 2, "hello")
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	if err := c.Delete(3, now); !errors.Is(err, ErrNotAuthor) || c.Deleted() {
		t.Errorf("Delete by another = %v, deleted %v", err, c.Deleted())
	}
	if err := c.Delete(2, now); err != nil || !c.Deleted() || c.Body != "" || !c.DeletedAt.Equal(now) {
		t.Errorf("Delete by author = %v, %+v", err, c)
	}
	if err := c.Delete(2, now); !errors.Is(err, ErrCommentDeleted) {
		t.Errorf("Delete twice = %v", err)
	}
	if err := c.Edit(2, "back"); !errors.Is(err, ErrCommentDeleted) {
		t.Errorf("Edit after delete = %v", err)
	}
}
