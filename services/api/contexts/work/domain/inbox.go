package domain

import "time"

// ReadMark is when a Member last read an Issue. An Issue is Unread to the
// Member when someone else commented on it after this.
type ReadMark struct {
	IssueID  uint64
	MemberID uint64
	At       time.Time
}

// InboxArchive is a Member taking an Issue out of their Inbox's Mine tab
// until it Resurfaces.
type InboxArchive struct {
	IssueID  uint64
	MemberID uint64
	At       time.Time
}

// ResurfaceStatuses are the statuses that bring an archived Issue back to
// Mine when it moves to one of them after the Inbox archive, as in
// Paperclip.
var ResurfaceStatuses = []IssueStatus{InReview, Blocked, Done}
