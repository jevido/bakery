package app

import (
	"context"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

// Templates is the catalog of Service templates, sorted by name.
func (s *Service) Templates() []domain.Template { return s.catalog }

// Template returns the template with the key.
func (s *Service) Template(key string) (domain.Template, bool) {
	for _, t := range s.catalog {
		if t.Key == key {
			return t, true
		}
	}
	return domain.Template{}, false
}

// CreateFromTemplate creates a Service from a template's Compose file; an
// empty name is the template's.
func (s *Service) CreateFromTemplate(ctx context.Context, environmentID uint64, key, name string) (View, error) {
	t, ok := s.Template(key)
	if !ok {
		return View{}, &domain.FieldError{Field: "template", Message: "there is no template " + key}
	}
	if name == "" {
		name = t.Name
	}
	return s.Create(ctx, environmentID, Input{Name: name, Compose: t.Compose, TemplateKey: t.Key})
}
