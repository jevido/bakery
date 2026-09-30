// Package infra is the servers context's persistence and how it reaches
// Servers (the local Podman socket, or SSH).
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/servers/domain"
)

type serverRecord struct {
	ID                   uint64 `gorm:"primaryKey"`
	Name                 string
	Kind                 string
	Host                 string
	Port                 int
	UserName             string
	PublicKey            string
	PrivateKeyEncrypted  string
	HostKey              string
	Status               string
	Validation           string
	LastCleanupAt        *time.Time
	LastCleanupReclaimed int64
	orm.Timestamps
}

func (serverRecord) TableName() string { return "servers" }

type checkJSON struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

type validationJSON struct {
	Checks    []checkJSON `json:"checks"`
	CheckedAt *time.Time  `json:"checked_at,omitempty"`
}

func toRecord(s domain.Server) (serverRecord, error) {
	private := ""
	if s.Key.Private != "" {
		var err error
		if private, err = facades.Crypt().EncryptString(s.Key.Private); err != nil {
			return serverRecord{}, err
		}
	}
	v := validationJSON{Checks: []checkJSON{}}
	for _, c := range s.Validation.Checks {
		v.Checks = append(v.Checks, checkJSON(c))
	}
	if !s.Validation.CheckedAt.IsZero() {
		at := s.Validation.CheckedAt.UTC()
		v.CheckedAt = &at
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return serverRecord{}, err
	}
	rec := serverRecord{
		ID: s.ID, Name: s.Name, Kind: string(s.Kind), Host: s.Host, Port: s.Port, UserName: s.User,
		PublicKey: s.Key.Public, PrivateKeyEncrypted: private, HostKey: s.HostKey,
		Status: string(s.Status), Validation: string(raw), LastCleanupReclaimed: s.LastCleanup.Reclaimed,
	}
	if !s.LastCleanup.At.IsZero() {
		at := s.LastCleanup.At.UTC()
		rec.LastCleanupAt = &at
	}
	return rec, nil
}

func (r serverRecord) toDomain() (domain.Server, error) {
	private := ""
	if r.PrivateKeyEncrypted != "" {
		var err error
		if private, err = facades.Crypt().DecryptString(r.PrivateKeyEncrypted); err != nil {
			return domain.Server{}, err
		}
	}
	s := domain.Server{
		ID: r.ID, Name: r.Name, Kind: domain.Kind(r.Kind), Host: r.Host, Port: r.Port, User: r.UserName,
		Key: domain.ServerKey{Public: r.PublicKey, Private: private}, HostKey: r.HostKey,
		Status: domain.Status(r.Status), LastCleanup: domain.Cleanup{Reclaimed: r.LastCleanupReclaimed},
		CreatedAt: createdAt(r.Timestamps),
	}
	if r.LastCleanupAt != nil {
		s.LastCleanup.At = r.LastCleanupAt.UTC()
	}
	var v validationJSON
	if r.Validation != "" {
		if err := json.Unmarshal([]byte(r.Validation), &v); err != nil {
			return domain.Server{}, err
		}
	}
	for _, c := range v.Checks {
		s.Validation.Checks = append(s.Validation.Checks, domain.Check(c))
	}
	if v.CheckedAt != nil {
		s.Validation.CheckedAt = v.CheckedAt.UTC()
	}
	return s, nil
}

// Store keeps Servers; private keys are encrypted with the application key.
type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func (s Store) Create(ctx context.Context, srv domain.Server) (domain.Server, error) {
	rec, err := toRecord(srv)
	if err != nil {
		return domain.Server{}, err
	}
	if err := s.query(ctx).Create(&rec); err != nil {
		return domain.Server{}, err
	}
	srv.ID = rec.ID
	srv.CreatedAt = createdAt(rec.Timestamps)
	return srv, nil
}

// Get returns the Server; found is false when there is none.
func (s Store) Get(ctx context.Context, id uint64) (domain.Server, bool, error) {
	var rec serverRecord
	if err := s.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Server{}, false, nil
		}
		return domain.Server{}, false, err
	}
	srv, err := rec.toDomain()
	return srv, err == nil, err
}

// List returns every Server, the Local server first, then oldest first.
func (s Store) List(ctx context.Context) ([]domain.Server, error) {
	var recs []serverRecord
	if err := s.query(ctx).OrderByRaw("kind = 'local' DESC, id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Server, len(recs))
	for i, r := range recs {
		srv, err := r.toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = srv
	}
	return out, nil
}

func (s Store) Save(ctx context.Context, srv domain.Server) error {
	rec, err := toRecord(srv)
	if err != nil {
		return err
	}
	_, err = s.query(ctx).Model(&serverRecord{}).Where("id", srv.ID).Update(map[string]any{
		"name": rec.Name, "host": rec.Host, "port": rec.Port, "user_name": rec.UserName,
		"host_key": rec.HostKey, "status": rec.Status, "validation": rec.Validation,
		"last_cleanup_at": rec.LastCleanupAt, "last_cleanup_reclaimed": rec.LastCleanupReclaimed,
	})
	return err
}

func (s Store) Delete(ctx context.Context, id uint64) error {
	_, err := s.query(ctx).Where("id", id).Delete(&serverRecord{})
	return err
}

// NameTaken reports whether a Server other than exceptID has the name
// (ignoring case).
func (s Store) NameTaken(ctx context.Context, name string, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&serverRecord{}).Where("lower(name) = lower(?)", name).Where("id <> ?", exceptID).Count()
	return n > 0, err
}

// AddressTaken reports whether a Server other than exceptID is reached as
// the same user on the same host and port.
func (s Store) AddressTaken(ctx context.Context, host string, port int, user string, exceptID uint64) (bool, error) {
	n, err := s.query(ctx).Model(&serverRecord{}).Where("lower(host) = lower(?)", host).Where("port", port).
		Where("user_name", user).Where("id <> ?", exceptID).Count()
	return n > 0, err
}

// Local returns the Local server; found is false before EnsureLocal.
func (s Store) Local(ctx context.Context) (domain.Server, bool, error) {
	var rec serverRecord
	if err := s.query(ctx).Where("kind", string(domain.Local)).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Server{}, false, nil
		}
		return domain.Server{}, false, err
	}
	srv, err := rec.toDomain()
	return srv, err == nil, err
}

func createdAt(t orm.Timestamps) time.Time {
	if t.CreatedAt == nil {
		return time.Time{}
	}
	return t.CreatedAt.StdTime().UTC()
}
