package infra

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
	"github.com/jevido/bakery/services/api/contexts/services/templates"
)

// Templates reads the embedded catalog, sorted by name.
func Templates() ([]domain.Template, error) {
	return ReadTemplates(templates.Files)
}

// ReadTemplates reads every *.yaml file in fsys as a Service template.
func ReadTemplates(fsys fs.FS) ([]domain.Template, error) {
	names, err := fs.Glob(fsys, "*.yaml")
	if err != nil {
		return nil, err
	}
	out := make([]domain.Template, 0, len(names))
	for _, name := range names {
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		t, err := domain.ParseTemplate(strings.TrimSuffix(name, ".yaml"), src)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b domain.Template) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}
