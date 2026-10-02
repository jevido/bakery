// Package infra stores Deployments and their logs with the Goravel ORM,
// clones with git, and builds and runs with Podman.
package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type deploymentRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	Preview       int
	ServerID      uint64
	Status        string
	Trigger       string
	Branch        string
	CommitSha     string
	CommitMessage string
	CommitAuthor  string
	SourceImage   string
	Image         string
	ContainerName string
	RollbackOf    *uint64
	Error         string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (deploymentRecord) TableName() string { return "deployments" }

func (r deploymentRecord) toDomain() domain.Deployment {
	return domain.Deployment{
		ID: r.ID, ApplicationID: r.ApplicationID, Preview: r.Preview, ServerID: r.ServerID, Status: domain.Status(r.Status), Trigger: domain.Trigger(r.Trigger),
		Branch: r.Branch, CommitSHA: r.CommitSha, CommitMessage: r.CommitMessage, CommitAuthor: r.CommitAuthor,
		SourceImage: r.SourceImage, Image: r.Image, Container: r.ContainerName, RollbackOf: r.RollbackOf, Error: r.Error, CreatedAt: r.CreatedAt,
		StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

const (
	activeList  = "('queued', 'cloning', 'building', 'starting')"
	runningList = "('cloning', 'building', 'starting')"
)

type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Store) Queue(ctx context.Context, d domain.Deployment) (domain.Deployment, error) {
	now := time.Now()
	rec := deploymentRecord{
		ApplicationID: d.ApplicationID, Preview: d.Preview, ServerID: d.ServerID, Status: string(domain.Queued), Trigger: string(d.Trigger),
		Branch: d.Branch, CommitSha: d.CommitSHA, CommitMessage: d.CommitMessage, CommitAuthor: d.CommitAuthor,
		SourceImage: d.SourceImage, Image: d.Image, RollbackOf: d.RollbackOf, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		if strings.Contains(err.Error(), "deployments_one_queued") {
			return domain.Deployment{}, domain.ErrAlreadyQueued
		}
		return domain.Deployment{}, err
	}
	return rec.toDomain(), nil
}

// ClaimNext is one statement: the inner SELECT locks the oldest queued row
// of an Application with nothing running and skips rows another worker
// holds, so two workers never take the same Deployment. Two workers claiming
// two queued rows of one Application cannot happen either (one queued per
// Application); should it race anyway, deployments_one_running refuses the
// second, which is reported as nothing to claim.
func (s Store) ClaimNext(ctx context.Context) (domain.Deployment, bool, error) {
	var recs []deploymentRecord
	err := s.query(ctx).Raw(`
		UPDATE deployments SET status = 'cloning', started_at = now(), updated_at = now()
		WHERE id = (
			SELECT q.id FROM deployments q
			WHERE q.status = 'queued' AND NOT EXISTS (
				SELECT 1 FROM deployments r
				WHERE r.application_id = q.application_id AND r.status IN ` + runningList + `
			)
			ORDER BY q.id FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING *`).Scan(&recs)
	if err != nil && strings.Contains(err.Error(), "deployments_one_running") {
		return domain.Deployment{}, false, nil
	}
	if err != nil || len(recs) == 0 {
		return domain.Deployment{}, false, err
	}
	return recs[0].toDomain(), true, nil
}

func (s Store) Save(ctx context.Context, d domain.Deployment) error {
	_, err := s.query(ctx).Model(&deploymentRecord{}).Where("id", d.ID).Update(map[string]any{
		"status": string(d.Status), "server_id": d.ServerID, "branch": d.Branch, "commit_sha": d.CommitSHA,
		"commit_message": d.CommitMessage, "commit_author": d.CommitAuthor, "source_image": d.SourceImage, "image": d.Image,
		"container_name": d.Container, "error": d.Error, "finished_at": d.FinishedAt, "updated_at": time.Now(),
	})
	return err
}

func (s Store) CancelQueued(ctx context.Context, id uint64) (bool, error) {
	res, err := s.query(ctx).Exec(`
		UPDATE deployments SET status = 'cancelled', finished_at = now(), updated_at = now()
		WHERE id = ? AND status = 'queued'`, id)
	if err != nil {
		return false, err
	}
	return res.RowsAffected == 1, nil
}

func (s Store) FailInterrupted(ctx context.Context, reason string) (int, error) {
	res, err := s.query(ctx).Exec(`
		UPDATE deployments SET status = 'failed', error = ?, finished_at = now(), updated_at = now()
		WHERE status IN ('cloning', 'building', 'starting')`, reason)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected), nil
}

func (s Store) ByID(ctx context.Context, id uint64) (domain.Deployment, bool, error) {
	var rec deploymentRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Deployment{}, false, nil
		}
		return domain.Deployment{}, false, err
	}
	return rec.toDomain(), true, nil
}

func (s Store) ByApplication(ctx context.Context, applicationID uint64, limit int) ([]domain.Deployment, error) {
	var recs []deploymentRecord
	if err := s.query(ctx).Where("application_id", applicationID).OrderByDesc("id").Limit(limit).Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Deployment, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

// ByPreview returns the newest Deployments of one Preview of the
// Application, newest first.
func (s Store) ByPreview(ctx context.Context, applicationID uint64, number int, limit int) ([]domain.Deployment, error) {
	var recs []deploymentRecord
	if err := s.query(ctx).Where("application_id", applicationID).Where("preview", number).OrderByDesc("id").Limit(limit).Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Deployment, len(recs))
	for i, r := range recs {
		out[i] = r.toDomain()
	}
	return out, nil
}

func (s Store) ApplicationIDs(ctx context.Context) ([]uint64, error) {
	var ids []uint64
	err := s.query(ctx).Model(&deploymentRecord{}).Distinct("application_id").Pluck("application_id", &ids)
	return ids, err
}

func (s Store) Active(ctx context.Context, applicationID uint64) (domain.Deployment, bool, error) {
	var recs []deploymentRecord
	if err := s.query(ctx).Where("application_id", applicationID).Where("status IN " + activeList).Limit(1).Find(&recs); err != nil {
		return domain.Deployment{}, false, err
	}
	if len(recs) == 0 {
		return domain.Deployment{}, false, nil
	}
	return recs[0].toDomain(), true, nil
}

func (s Store) ServerIDs(ctx context.Context, applicationID uint64) ([]uint64, error) {
	var ids []uint64
	err := s.query(ctx).Model(&deploymentRecord{}).Where("application_id", applicationID).Distinct("server_id").Pluck("server_id", &ids)
	return ids, err
}

func (s Store) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	_, err := s.query(ctx).Where("application_id", applicationID).Delete(&deploymentRecord{})
	return err
}
