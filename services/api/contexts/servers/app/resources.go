package app

import (
	"context"
	"maps"
	"slices"
	"strings"
)

// Resource is an Application, Database or Service on a Server, as the
// Server's Resources list shows it.
type Resource struct {
	// Type is "application", "database" or "service".
	Type          string
	ID            uint64
	Name          string
	ProjectID     uint64
	Project       string
	EnvironmentID uint64
	Environment   string
	Status        string
}

// ResourceProject is a Project with the names of its Environments, by id.
type ResourceProject struct {
	ID           uint64
	Name         string
	Environments map[uint64]string
}

// ResourceLister lists one Type of Resource on the Server (0 for the Local
// server) in the Projects' Environments, filling ID, Name, EnvironmentID and
// Status.
type ResourceLister func(ctx context.Context, serverID uint64, projects []ResourceProject) ([]Resource, error)

// OnResources registers the lister of one Type of Resource.
func (s *Service) OnResources(kind string, list ResourceLister) {
	if s.listers == nil {
		s.listers = map[string]ResourceLister{}
	}
	s.listers[kind] = list
}

// Resources lists every registered Type of Resource on the Server in the
// Projects, sorted by name.
func (s *Service) Resources(ctx context.Context, id uint64, projects []ResourceProject) ([]Resource, error) {
	srv, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	where := map[uint64]ResourceProject{}
	for _, p := range projects {
		for envID := range p.Environments {
			where[envID] = p
		}
	}
	out := []Resource{}
	for _, kind := range slices.Sorted(maps.Keys(s.listers)) {
		list, err := s.listers[kind](ctx, srv.RefID(), projects)
		if err != nil {
			return nil, err
		}
		for _, r := range list {
			p, ok := where[r.EnvironmentID]
			if !ok {
				continue
			}
			r.Type, r.ProjectID, r.Project, r.Environment = kind, p.ID, p.Name, p.Environments[r.EnvironmentID]
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b Resource) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}
