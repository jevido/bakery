package infra

import (
	"context"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/work/app"
	"github.com/jevido/bakery/services/api/contexts/work/domain"
)

type documentRecord struct {
	ID                uint64
	GuildID           uint64
	IssueID           uint64
	Key               string
	Title             string
	Body              string
	RevisionNumber    int
	LatestRevisionID  uint64
	CreatedByMemberID *uint64
	UpdatedByMemberID *uint64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (r documentRecord) toDomain() domain.IssueDocument {
	return domain.IssueDocument{
		ID: r.ID, GuildID: r.GuildID, IssueID: r.IssueID, Key: r.Key, Title: r.Title, Body: r.Body,
		Latest: r.RevisionNumber, LatestRevisionID: r.LatestRevisionID,
		CreatedByID: deref(r.CreatedByMemberID), UpdatedByID: deref(r.UpdatedByMemberID),
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

type revisionRecord struct {
	ID                uint64
	DocumentID        uint64
	Number            int
	Title             string
	Body              string
	ChangeSummary     *string
	CreatedByMemberID *uint64
	CreatedAt         time.Time
}

func (r revisionRecord) toDomain() domain.Revision {
	rev := domain.Revision{
		ID: r.ID, DocumentID: r.DocumentID, Number: r.Number, Title: r.Title, Body: r.Body,
		AuthorID: deref(r.CreatedByMemberID), CreatedAt: r.CreatedAt.UTC(),
	}
	if r.ChangeSummary != nil {
		rev.Summary = *r.ChangeSummary
	}
	return rev
}

func summary(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// selectDocuments reads documents with the id of their newest Revision.
const selectDocuments = `SELECT d.*, r.id AS latest_revision_id FROM issue_documents d
	JOIN issue_document_revisions r ON r.document_id = d.id AND r.number = d.revision_number `

// Documents keeps Issue documents and their Revisions.
type Documents struct{}

func (Documents) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Documents) Documents(ctx context.Context, issueID uint64) ([]domain.IssueDocument, error) {
	var recs []documentRecord
	if err := s.query(ctx).Raw(selectDocuments+`WHERE d.issue_id = ? ORDER BY d.key`, issueID).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.IssueDocument, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (Documents) document(q contractsorm.Query, issueID uint64, key string) (domain.IssueDocument, bool, error) {
	var recs []documentRecord
	if err := q.Raw(selectDocuments+`WHERE d.issue_id = ? AND d.key = ?`, issueID, key).Scan(&recs); err != nil {
		return domain.IssueDocument{}, false, err
	}
	if len(recs) == 0 {
		return domain.IssueDocument{}, false, nil
	}
	return recs[0].toDomain(), true, nil
}

func (s Documents) Document(ctx context.Context, issueID uint64, key string) (domain.IssueDocument, bool, error) {
	return s.document(s.query(ctx), issueID, key)
}

func (s Documents) Revisions(ctx context.Context, documentID uint64) ([]domain.Revision, error) {
	var recs []revisionRecord
	if err := s.query(ctx).Raw(`SELECT * FROM issue_document_revisions WHERE document_id = ? ORDER BY number DESC`, documentID).Scan(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Revision, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s Documents) Revision(ctx context.Context, documentID, id uint64) (domain.Revision, bool, error) {
	var recs []revisionRecord
	if err := s.query(ctx).Raw(`SELECT * FROM issue_document_revisions WHERE document_id = ? AND id = ?`, documentID, id).Scan(&recs); err != nil {
		return domain.Revision{}, false, err
	}
	if len(recs) == 0 {
		return domain.Revision{}, false, nil
	}
	return recs[0].toDomain(), true, nil
}

// addRevision stores r as a Revision of document id and touches the
// Issue's updated_at, so the Issue sorts up in lists. The Issue's rules do
// not depend on updated_at, so this does not change the Issue as an
// aggregate.
func addRevision(tx contractsorm.Query, id, issueID uint64, r domain.Revision) error {
	if _, err := tx.Exec(`INSERT INTO issue_document_revisions (document_id, number, title, body, change_summary, created_by_member_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, r.Number, r.Title, r.Body, summary(r.Summary), nullable(r.AuthorID), r.CreatedAt); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE issues SET updated_at = now() WHERE id = ?`, issueID)
	return err
}

// CreateDocument inserts the document unless the Issue has its key
// already, and its first Revision, in one transaction.
func (s Documents) CreateDocument(ctx context.Context, d domain.IssueDocument, r domain.Revision) (domain.IssueDocument, error) {
	var saved domain.IssueDocument
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		var ids []uint64
		if err := tx.Raw(`INSERT INTO issue_documents (guild_id, issue_id, key, title, body, revision_number, created_by_member_id, updated_by_member_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (issue_id, key) DO NOTHING RETURNING id`,
			d.GuildID, d.IssueID, d.Key, d.Title, d.Body, d.Latest, nullable(d.CreatedByID), nullable(d.UpdatedByID), d.CreatedAt, d.UpdatedAt).Scan(&ids); err != nil {
			return err
		}
		if len(ids) == 0 {
			return app.ErrDocumentExists
		}
		if err := addRevision(tx, ids[0], d.IssueID, r); err != nil {
			return err
		}
		var err error
		saved, _, err = s.document(tx, d.IssueID, d.Key)
		return err
	})
	return saved, err
}

// SaveRevision moves the document to r only while it is still at the
// Revision before r, so of two saves on the same Base revision the second
// is refused.
func (s Documents) SaveRevision(ctx context.Context, d domain.IssueDocument, r domain.Revision) (domain.IssueDocument, error) {
	var saved domain.IssueDocument
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		res, err := tx.Exec(`UPDATE issue_documents SET title = ?, body = ?, revision_number = ?, updated_by_member_id = ?, updated_at = ?
			WHERE id = ? AND revision_number = ?`, d.Title, d.Body, r.Number, nullable(d.UpdatedByID), d.UpdatedAt, d.ID, r.Number-1)
		if err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return domain.ErrStaleRevision
		}
		if err := addRevision(tx, d.ID, d.IssueID, r); err != nil {
			return err
		}
		saved, _, err = s.document(tx, d.IssueID, d.Key)
		return err
	})
	return saved, err
}

func (s Documents) DeleteDocument(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Exec(`DELETE FROM issue_documents WHERE id = ?`, id)
	return err
}
