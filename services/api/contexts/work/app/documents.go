package app

import (
	"context"
	"errors"

	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

var (
	// ErrDocumentExists is a new Issue document under a Document key the
	// Issue already has; someone created it in between.
	ErrDocumentExists = errors.New("document key already exists on this issue")
	// ErrNoDocumentYet is a save with a Base revision for a Document key
	// the Issue has no document under.
	ErrNoDocumentYet = errors.New("document does not exist yet")
)

// StaleRevisionError is a save or Restore refused because Current's newest
// Revision is not the Base revision it started from.
type StaleRevisionError struct {
	Current domain.IssueDocument
}

func (e *StaleRevisionError) Error() string { return domain.ErrStaleRevision.Error() }
func (e *StaleRevisionError) Unwrap() error { return domain.ErrStaleRevision }

// Documents keeps Issue documents and their Revisions.
type Documents interface {
	// Documents lists the Issue's documents by key.
	Documents(ctx context.Context, issueID uint64) ([]domain.IssueDocument, error)
	Document(ctx context.Context, issueID uint64, key string) (domain.IssueDocument, bool, error)
	// Revisions lists the document's Revisions, newest first.
	Revisions(ctx context.Context, documentID uint64) ([]domain.Revision, error)
	Revision(ctx context.Context, documentID, id uint64) (domain.Revision, bool, error)
	// CreateDocument stores a new document with its first Revision, or
	// answers ErrDocumentExists for a key the Issue already has.
	CreateDocument(ctx context.Context, d domain.IssueDocument, r domain.Revision) (domain.IssueDocument, error)
	// SaveRevision stores the document and its new newest Revision r, or
	// answers domain.ErrStaleRevision when the stored document is no longer
	// at r.Number-1.
	SaveRevision(ctx context.Context, d domain.IssueDocument, r domain.Revision) (domain.IssueDocument, error)
	// DeleteDocument deletes the document and its Revisions.
	DeleteDocument(ctx context.Context, id uint64) error
}

// DocumentInput is an Issue document as saved. BaseRevisionID is the id
// of the Revision the edit started from, 0 for a new document.
type DocumentInput struct {
	Title          string
	Body           string
	ChangeSummary  string
	BaseRevisionID uint64
}

// Documents lists the documents of an Issue the person may see.
func (s *Service) Documents(ctx context.Context, guildID uint64, ref string, visible Visible) ([]domain.IssueDocument, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return nil, err
	}
	return s.docs.Documents(ctx, i.ID)
}

// document finds the Issue's document under key, with the Issue; an
// unknown key is ErrNotFound and a malformed one a FieldError.
func (s *Service) document(ctx context.Context, guildID uint64, ref, key string, visible Visible) (domain.Issue, domain.IssueDocument, error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.Issue{}, domain.IssueDocument{}, err
	}
	k, err := domain.ParseDocumentKey(key)
	if err != nil {
		return domain.Issue{}, domain.IssueDocument{}, err
	}
	d, found, err := s.docs.Document(ctx, i.ID, k)
	if err != nil {
		return domain.Issue{}, domain.IssueDocument{}, err
	}
	if !found {
		return domain.Issue{}, domain.IssueDocument{}, ErrNotFound
	}
	return i, d, nil
}

// Document is one Issue document.
func (s *Service) Document(ctx context.Context, guildID uint64, ref, key string, visible Visible) (domain.IssueDocument, error) {
	_, d, err := s.document(ctx, guildID, ref, key, visible)
	return d, err
}

// stale answers a refused save with the document as it now is.
func (s *Service) stale(ctx context.Context, d domain.IssueDocument) error {
	current, found, err := s.docs.Document(ctx, d.IssueID, d.Key)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return &StaleRevisionError{Current: current}
}

// SaveDocument creates the Issue's document under key, or saves a new
// Revision of it on top of the Base revision. created tells which.
func (s *Service) SaveDocument(ctx context.Context, guildID, memberID uint64, ref, key string, in DocumentInput, visible Visible) (d domain.IssueDocument, created bool, err error) {
	i, err := s.Issue(ctx, guildID, ref, visible)
	if err != nil {
		return domain.IssueDocument{}, false, err
	}
	k, err := domain.ParseDocumentKey(key)
	if err != nil {
		return domain.IssueDocument{}, false, err
	}
	d, found, err := s.docs.Document(ctx, i.ID, k)
	if err != nil {
		return domain.IssueDocument{}, false, err
	}
	now := s.now()
	if !found {
		if in.BaseRevisionID != 0 {
			return domain.IssueDocument{}, false, ErrNoDocumentYet
		}
		d, r, err := domain.NewIssueDocument(i.GuildID, i.ID, k, in.Title, in.Body, in.ChangeSummary, memberID, now)
		if err != nil {
			return domain.IssueDocument{}, false, err
		}
		if d, err = s.docs.CreateDocument(ctx, d, r); err != nil {
			return domain.IssueDocument{}, false, err
		}
		s.publish(ctx, domain.DocumentSaved{Happened: s.happened(memberID), Issue: i, Document: d, First: true})
		return d, true, nil
	}
	// The Base revision travels as an id; one that is not this document's
	// is no Revision of it, so stale.
	base := 0
	if in.BaseRevisionID != 0 {
		r, ok, err := s.docs.Revision(ctx, d.ID, in.BaseRevisionID)
		if err != nil {
			return domain.IssueDocument{}, false, err
		}
		if ok {
			base = r.Number
		}
	}
	r, err := d.Save(in.Title, in.Body, in.ChangeSummary, base, memberID, now)
	if errors.Is(err, domain.ErrStaleRevision) {
		return domain.IssueDocument{}, false, &StaleRevisionError{Current: d}
	}
	if err != nil {
		return domain.IssueDocument{}, false, err
	}
	saved, err := s.docs.SaveRevision(ctx, d, r)
	if errors.Is(err, domain.ErrStaleRevision) {
		return domain.IssueDocument{}, false, s.stale(ctx, d)
	}
	if err != nil {
		return domain.IssueDocument{}, false, err
	}
	s.publish(ctx, domain.DocumentSaved{Happened: s.happened(memberID), Issue: i, Document: saved})
	return saved, false, nil
}

// DeleteDocument deletes the Issue's document and its Revisions, by the
// Member.
func (s *Service) DeleteDocument(ctx context.Context, guildID, memberID uint64, ref, key string, visible Visible) error {
	i, d, err := s.document(ctx, guildID, ref, key, visible)
	if err != nil {
		return err
	}
	if err := s.docs.DeleteDocument(ctx, d.ID); err != nil {
		return err
	}
	s.publish(ctx, domain.DocumentDeleted{Happened: s.happened(memberID), Issue: i, Document: d})
	return nil
}

// Revisions lists a document's Revisions, newest first.
func (s *Service) Revisions(ctx context.Context, guildID uint64, ref, key string, visible Visible) ([]domain.Revision, error) {
	_, d, err := s.document(ctx, guildID, ref, key, visible)
	if err != nil {
		return nil, err
	}
	return s.docs.Revisions(ctx, d.ID)
}

// RestoreRevision saves an older Revision of the document as its newest.
func (s *Service) RestoreRevision(ctx context.Context, guildID, memberID uint64, ref, key string, revisionID uint64, visible Visible) (domain.IssueDocument, error) {
	i, d, err := s.document(ctx, guildID, ref, key, visible)
	if err != nil {
		return domain.IssueDocument{}, err
	}
	old, found, err := s.docs.Revision(ctx, d.ID, revisionID)
	if err != nil {
		return domain.IssueDocument{}, err
	}
	if !found {
		return domain.IssueDocument{}, ErrNotFound
	}
	r, err := d.Restore(old, memberID, s.now())
	if err != nil {
		return domain.IssueDocument{}, err
	}
	saved, err := s.docs.SaveRevision(ctx, d, r)
	if errors.Is(err, domain.ErrStaleRevision) {
		return domain.IssueDocument{}, s.stale(ctx, d)
	}
	if err != nil {
		return domain.IssueDocument{}, err
	}
	s.publish(ctx, domain.DocumentSaved{Happened: s.happened(memberID), Issue: i, Document: saved, RestoredFrom: old.Number})
	return saved, nil
}
