package app

import (
	"context"
	"errors"
	"time"

	"github.com/jevido/bakery/services/api/contexts/databases/domain"
)

// ErrS3StorageInUse is returned when deleting an S3 storage a Backup execution
// schedule uses.
var ErrS3StorageInUse = errors.New("a database's backup schedule uses this S3 storage: choose another storage there first")

// checkTimeout bounds one connection test.
const checkTimeout = 15 * time.Second

func (s *Service) S3Storages(ctx context.Context) ([]domain.S3Storage, error) {
	return s.store.S3Storages(ctx)
}

func (s *Service) s3Storage(ctx context.Context, id uint64) (domain.S3Storage, error) {
	st, found, err := s.store.S3Storage(ctx, id)
	if err == nil && !found {
		err = ErrNotFound
	}
	return st, err
}

func (s *Service) checkS3Name(ctx context.Context, st domain.S3Storage) error {
	taken, err := s.store.S3StorageNameTaken(ctx, st.Name, st.ID)
	if err != nil {
		return err
	}
	if taken {
		return &domain.FieldError{Field: "name", Message: "another S3 storage has this name"}
	}
	return nil
}

func (s *Service) CreateS3Storage(ctx context.Context, in domain.S3Input) (domain.S3Storage, error) {
	st, err := domain.NewS3Storage(in)
	if err != nil {
		return st, err
	}
	if err := s.checkS3Name(ctx, st); err != nil {
		return st, err
	}
	return s.store.CreateS3Storage(ctx, st)
}

// UpdateS3Storage changes the S3 storage; an empty secret key keeps it.
func (s *Service) UpdateS3Storage(ctx context.Context, id uint64, in domain.S3Input) (domain.S3Storage, error) {
	st, err := s.s3Storage(ctx, id)
	if err != nil {
		return st, err
	}
	if err := st.Update(in); err != nil {
		return st, err
	}
	if err := s.checkS3Name(ctx, st); err != nil {
		return st, err
	}
	return st, s.store.SaveS3Storage(ctx, st)
}

func (s *Service) DeleteS3Storage(ctx context.Context, id uint64) error {
	if _, err := s.s3Storage(ctx, id); err != nil {
		return err
	}
	used, err := s.store.S3StorageInUse(ctx, id)
	if err != nil {
		return err
	}
	if used {
		return ErrS3StorageInUse
	}
	return s.store.DeleteS3Storage(ctx, id)
}

// CheckS3Storage tests whether Bakery reaches the bucket with the input,
// before it is saved. With id, an empty secret key means the stored one.
// The returned error is the storage's answer; a broken input is a
// FieldError.
func (s *Service) CheckS3Storage(ctx context.Context, id uint64, in domain.S3Input) (connErr error, err error) {
	var st domain.S3Storage
	if id != 0 {
		if st, err = s.s3Storage(ctx, id); err != nil {
			return nil, err
		}
	}
	if err := st.Update(in); err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return s.S3(st).Check(cctx), nil
}
