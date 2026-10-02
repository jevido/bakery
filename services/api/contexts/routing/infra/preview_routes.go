package infra

import (
	"context"
	"encoding/json"

	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type previewRouteRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	Preview       int
	ServerID      uint64
	Domains       string // JSON array
	ContainerName string
	ContainerPort int
	orm.Timestamps
}

func (previewRouteRecord) TableName() string { return "preview_routes" }

// PreviewRoutes stores Preview routes, one per (Application, Preview).
type PreviewRoutes struct{}

func (PreviewRoutes) All(ctx context.Context) ([]domain.PreviewRoute, error) {
	var recs []previewRouteRecord
	if err := facades.Orm().WithContext(ctx).Query().OrderBy("application_id").OrderBy("preview").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.PreviewRoute, len(recs))
	for i, r := range recs {
		out[i] = domain.PreviewRoute{ApplicationID: r.ApplicationID, Preview: r.Preview, ServerID: r.ServerID, Container: r.ContainerName, Port: r.ContainerPort}
		if err := json.Unmarshal([]byte(r.Domains), &out[i].Domains); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (PreviewRoutes) Upsert(ctx context.Context, r domain.PreviewRoute) error {
	domains, err := json.Marshal(r.Domains)
	if err != nil {
		return err
	}
	_, err = facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO preview_routes (application_id, preview, server_id, domains, container_name, container_port, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id, preview) DO UPDATE
		SET server_id = EXCLUDED.server_id, domains = EXCLUDED.domains, container_name = EXCLUDED.container_name,
		    container_port = EXCLUDED.container_port, updated_at = now()`,
		r.ApplicationID, r.Preview, r.ServerID, string(domains), r.Container, r.Port)
	return err
}

func (PreviewRoutes) Delete(ctx context.Context, applicationID uint64, preview int) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Where("preview", preview).Delete(&previewRouteRecord{})
	return err
}

func (PreviewRoutes) DeleteForApplication(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&previewRouteRecord{})
	return err
}
