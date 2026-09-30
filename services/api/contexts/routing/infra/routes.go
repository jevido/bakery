package infra

import (
	"context"
	"encoding/json"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/routing/domain"
)

type routeRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ApplicationID uint64
	// Domains is a JSON array, primary first.
	Domains       string
	ContainerName string
	ContainerPort int
	// ServerID is the Server the Route is served on, 0 the Local server.
	ServerID uint64
	orm.Timestamps
}

func (routeRecord) TableName() string { return "routes" }

type Routes struct{}

func (Routes) All(ctx context.Context) ([]domain.Route, error) {
	var recs []routeRecord
	if err := facades.Orm().WithContext(ctx).Query().OrderBy("application_id").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.Route, len(recs))
	for i, r := range recs {
		out[i] = domain.Route{ApplicationID: r.ApplicationID, ServerID: r.ServerID, Container: r.ContainerName, Port: r.ContainerPort}
		if err := json.Unmarshal([]byte(r.Domains), &out[i].Domains); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Upsert keeps one Route per Application: the unique application_id index
// is the conflict target.
func (Routes) Upsert(ctx context.Context, r domain.Route) error {
	domains, err := json.Marshal(r.Domains)
	if err != nil {
		return err
	}
	_, err = facades.Orm().WithContext(ctx).Query().Exec(`
		INSERT INTO routes (application_id, server_id, domains, container_name, container_port, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (application_id) DO UPDATE
		SET server_id = EXCLUDED.server_id, domains = EXCLUDED.domains, container_name = EXCLUDED.container_name,
		    container_port = EXCLUDED.container_port, updated_at = now()`,
		r.ApplicationID, r.ServerID, string(domains), r.Container, r.Port)
	return err
}

// ChangeDomains points an existing Route at the Domains; found is false
// when the Application has no Route yet.
func (Routes) ChangeDomains(ctx context.Context, applicationID uint64, domains []string) (bool, error) {
	raw, err := json.Marshal(domains)
	if err != nil {
		return false, err
	}
	res, err := facades.Orm().WithContext(ctx).Query().Model(&routeRecord{}).Where("application_id", applicationID).Update("domains", string(raw))
	if err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}

func (Routes) Delete(ctx context.Context, applicationID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("application_id", applicationID).Delete(&routeRecord{})
	return err
}

type serviceRouteRecord struct {
	ID            uint64 `gorm:"primaryKey"`
	ServiceID     uint64
	Component     string
	Domains       string // JSON array, primary first
	ContainerName string
	ContainerPort int
	orm.Timestamps
}

func (serviceRouteRecord) TableName() string { return "service_routes" }

// ServiceRoutes stores Service routes, replaced per Service as a set.
type ServiceRoutes struct{}

func (ServiceRoutes) All(ctx context.Context) ([]domain.ServiceRoute, error) {
	var recs []serviceRouteRecord
	if err := facades.Orm().WithContext(ctx).Query().OrderBy("service_id").OrderBy("component").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.ServiceRoute, len(recs))
	for i, r := range recs {
		out[i] = domain.ServiceRoute{ServiceID: r.ServiceID, Component: r.Component, Container: r.ContainerName, Port: r.ContainerPort}
		if err := json.Unmarshal([]byte(r.Domains), &out[i].Domains); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Replace makes routes the Service's whole set, in one transaction.
func (ServiceRoutes) Replace(ctx context.Context, serviceID uint64, routes []domain.ServiceRoute) error {
	return facades.Orm().WithContext(ctx).Transaction(func(tx contractsorm.Query) error {
		if _, err := tx.Where("service_id", serviceID).Delete(&serviceRouteRecord{}); err != nil {
			return err
		}
		for _, r := range routes {
			domains, err := json.Marshal(r.Domains)
			if err != nil {
				return err
			}
			rec := serviceRouteRecord{ServiceID: serviceID, Component: r.Component, Domains: string(domains), ContainerName: r.Container, ContainerPort: r.Port}
			if err := tx.Create(&rec); err != nil {
				return err
			}
		}
		return nil
	})
}

func (ServiceRoutes) Delete(ctx context.Context, serviceID uint64) error {
	_, err := facades.Orm().WithContext(ctx).Query().Where("service_id", serviceID).Delete(&serviceRouteRecord{})
	return err
}
