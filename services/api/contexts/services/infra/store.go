// Package infra is the services context's persistence, value generators
// and Podman runtime.
package infra

import (
	"context"
	"encoding/json"
	"errors"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

type serviceRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	EnvironmentID uint64
	ProjectID     uint64
	Name          string
	Description   string
	Slug          string
	ComposeFile   string
	TemplateKey   string
	DesiredState  string
	LastError     string
	orm.Timestamps
}

func (serviceRecord) TableName() string { return "services" }

type componentRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	ServiceID uint64
	Name      string
	Image     string
	Public    bool
	Port      int
	Domains   string // JSON array, primary first
	Position  int
	orm.Timestamps
}

func (componentRecord) TableName() string { return "service_components" }

type variableRecord struct {
	ID             uint64 `gorm:"primaryKey"`
	ServiceID      uint64
	Name           string
	ValueEncrypted string
	Magic          string
	DefaultValue   string
	HasDefault     bool
	Position       int
	orm.Timestamps
}

func (variableRecord) TableName() string { return "service_variables" }

// Store keeps Services with their Components and Service variables; the
// variables' values are encrypted with the application key.
type Store struct{}

func (Store) query(ctx context.Context) contractsorm.Query {
	return facades.Orm().WithContext(ctx).Query()
}

func encrypt(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	return facades.Crypt().EncryptString(s)
}

func decrypt(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	return facades.Crypt().DecryptString(s)
}

func toRecord(s domain.Service) serviceRecord {
	return serviceRecord{
		ID: s.ID, EnvironmentID: s.EnvironmentID, ProjectID: s.ProjectID,
		Name: s.Name, Description: s.Description, Slug: s.Slug, ComposeFile: s.ComposeFile, TemplateKey: s.TemplateKey,
		DesiredState: string(s.DesiredState), LastError: s.LastError,
	}
}

// Create stores a new Service and returns it with its id.
func (st Store) Create(ctx context.Context, s domain.Service) (domain.Service, error) {
	rec := toRecord(s)
	err := facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if err := tx.Create(&rec); err != nil {
			return err
		}
		return writeParts(tx, rec.ID, s)
	})
	if err != nil {
		return domain.Service{}, err
	}
	s.ID = rec.ID
	return s, nil
}

// Save stores the whole aggregate: the row, its Components and its
// variables, in one transaction.
func (st Store) Save(ctx context.Context, s domain.Service) error {
	rec := toRecord(s)
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Model(&serviceRecord{}).Where("id", s.ID).Update(map[string]any{
			"name": rec.Name, "description": rec.Description, "compose_file": rec.ComposeFile, "desired_state": rec.DesiredState, "last_error": rec.LastError,
		}); err != nil {
			return err
		}
		if _, err := tx.Where("service_id", s.ID).Delete(&componentRecord{}); err != nil {
			return err
		}
		if _, err := tx.Where("service_id", s.ID).Delete(&variableRecord{}); err != nil {
			return err
		}
		return writeParts(tx, s.ID, s)
	})
}

func writeParts(tx contractsorm.Query, serviceID uint64, s domain.Service) error {
	for i, c := range s.Components {
		domains, err := json.Marshal(c.Domains)
		if err != nil {
			return err
		}
		if c.Domains == nil {
			domains = []byte("[]")
		}
		rec := componentRecord{ServiceID: serviceID, Name: c.Name, Image: c.Image, Public: c.Public, Port: c.Port, Domains: string(domains), Position: i}
		if err := tx.Create(&rec); err != nil {
			return err
		}
	}
	for i, v := range s.Variables {
		value, err := encrypt(v.Value)
		if err != nil {
			return err
		}
		rec := variableRecord{ServiceID: serviceID, Name: v.Name, ValueEncrypted: value, Magic: string(v.Magic), DefaultValue: v.Default, HasDefault: v.HasDefault, Position: i}
		if err := tx.Create(&rec); err != nil {
			return err
		}
	}
	return nil
}

// Get returns the Service; found is false when there is none.
func (st Store) Get(ctx context.Context, id uint64) (domain.Service, bool, error) {
	var rec serviceRecord
	if err := st.query(ctx).Where("id", id).FirstOrFail(&rec); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return domain.Service{}, false, nil
		}
		return domain.Service{}, false, err
	}
	out, err := st.withParts(ctx, []serviceRecord{rec})
	if err != nil {
		return domain.Service{}, false, err
	}
	return out[0], true, nil
}

func (st Store) list(ctx context.Context, q contractsorm.Query) ([]domain.Service, error) {
	var recs []serviceRecord
	if err := q.Order("id").Find(&recs); err != nil {
		return nil, err
	}
	return st.withParts(ctx, recs)
}

func (st Store) withParts(ctx context.Context, recs []serviceRecord) ([]domain.Service, error) {
	out := make([]domain.Service, len(recs))
	if len(recs) == 0 {
		return out, nil
	}
	ids := make([]any, len(recs))
	index := map[uint64]int{}
	for i, r := range recs {
		ids[i] = r.ID
		index[r.ID] = i
		out[i] = domain.Service{
			ID: r.ID, EnvironmentID: r.EnvironmentID, ProjectID: r.ProjectID,
			Name: r.Name, Description: r.Description, Slug: r.Slug, ComposeFile: r.ComposeFile, TemplateKey: r.TemplateKey,
			DesiredState: domain.DesiredState(r.DesiredState), LastError: r.LastError,
		}
	}
	var comps []componentRecord
	if err := st.query(ctx).WhereIn("service_id", ids).Order("position").Find(&comps); err != nil {
		return nil, err
	}
	for _, c := range comps {
		comp := domain.Component{Name: c.Name, Image: c.Image, Public: c.Public, Port: c.Port}
		if err := json.Unmarshal([]byte(c.Domains), &comp.Domains); err != nil {
			return nil, err
		}
		if len(comp.Domains) == 0 {
			comp.Domains = nil
		}
		s := &out[index[c.ServiceID]]
		s.Components = append(s.Components, comp)
	}
	var vars []variableRecord
	if err := st.query(ctx).WhereIn("service_id", ids).Order("position").Find(&vars); err != nil {
		return nil, err
	}
	for _, v := range vars {
		value, err := decrypt(v.ValueEncrypted)
		if err != nil {
			return nil, err
		}
		s := &out[index[v.ServiceID]]
		s.Variables = append(s.Variables, domain.Variable{
			Name: v.Name, Value: value, Magic: domain.MagicKind(v.Magic), Default: v.DefaultValue, HasDefault: v.HasDefault,
		})
	}
	return out, nil
}

// ForProject lists the Project's Services, oldest first.
func (st Store) ForProject(ctx context.Context, projectID uint64) ([]domain.Service, error) {
	return st.list(ctx, st.query(ctx).Where("project_id", projectID))
}

// Wanted lists every Service whose desired state is running.
func (st Store) Wanted(ctx context.Context) ([]domain.Service, error) {
	return st.list(ctx, st.query(ctx).Where("desired_state", string(domain.Running)))
}

// Delete removes the Service with its Components and variables.
func (st Store) Delete(ctx context.Context, id uint64) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Where("service_id", id).Delete(&componentRecord{}); err != nil {
			return err
		}
		if _, err := tx.Where("service_id", id).Delete(&variableRecord{}); err != nil {
			return err
		}
		_, err := tx.Where("id", id).Delete(&serviceRecord{})
		return err
	})
}

func (st Store) SlugTaken(ctx context.Context, slug string) (bool, error) {
	n, err := st.query(ctx).Model(&serviceRecord{}).Where("slug", slug).Count()
	return n > 0, err
}

func (st Store) CountForProject(ctx context.Context, projectID uint64) (int64, error) {
	return st.query(ctx).Model(&serviceRecord{}).Where("project_id", projectID).Count()
}

func (st Store) CountForEnvironment(ctx context.Context, environmentID uint64) (int64, error) {
	return st.query(ctx).Model(&serviceRecord{}).Where("environment_id", environmentID).Count()
}

// DomainTaken reports whether a Component of a Service other than
// exceptServiceID has the Domain.
func (st Store) DomainTaken(ctx context.Context, d string, exceptServiceID uint64) (bool, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return false, err
	}
	// domains is a JSON array in a text column; jsonb containment finds the
	// element exactly.
	n, err := st.query(ctx).Model(&componentRecord{}).
		Where("domains::jsonb @> ?::jsonb", "["+string(raw)+"]").
		Where("service_id <> ?", exceptServiceID).Count()
	return n > 0, err
}

// SetLastError stores why the Service's last action failed ("" when it
// did not), without touching the rest of the aggregate.
func (st Store) SetLastError(ctx context.Context, id uint64, msg string) error {
	_, err := st.query(ctx).Model(&serviceRecord{}).Where("id", id).Update("last_error", msg)
	return err
}

// SetDesiredState stores what the Owner asked the Service to be.
func (st Store) SetDesiredState(ctx context.Context, id uint64, state domain.DesiredState) error {
	_, err := st.query(ctx).Model(&serviceRecord{}).Where("id", id).Update("desired_state", string(state))
	return err
}
