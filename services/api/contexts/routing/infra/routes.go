package infra

import (
	"context"

	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type routeRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	Domain        string
	ContainerName string
	ContainerPort int
	orm.Timestamps
}

func (routeRecord) TableName() string { return "routes" }

type Routes struct{}

func (Routes) All(ctx context.Context) ([]domain.Route, error) {
	var recs []routeRecord
	if err := facades.Orm().WithContext(ctx).Query().OrderBy("domain").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Route, len(recs))
	for i, r := range recs {
		out[i] = domain.Route{ApplicationID: r.ApplicationID, Domain: r.Domain, Container: r.ContainerName, Port: r.ContainerPort}
	}
	return out, nil
}

// Upsert keeps one Route per Application: the unique application_id index
// is the conflict target.
func (Routes) Upsert(ctx context.Context, r domain.Route) error {
	_, err := facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO routes (application_id, domain, container_name, container_port, created_at, updated_at)
		VALUES (?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET domain = EXCLUDED.domain, container_name = EXCLUDED.container_name,
		    container_port = EXCLUDED.container_port, updated_at = now()`,
		r.ApplicationID, r.Domain, r.Container, r.Port)
	return err
}

func (Routes) Delete(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&routeRecord{})
	return err
}
