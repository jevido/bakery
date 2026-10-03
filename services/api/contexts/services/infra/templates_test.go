package infra

import (
	"testing"

	"github.com/jevido/bakery/services/api/contexts/services/domain"
)

// Every template in the catalog parses, has exactly one Public Component
// and resolves once Bakery has filled in its Magic variables.
func TestCatalog(t *testing.T) {
	list, err := Templates()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"whoami": "whoami", "uptime-kuma": "uptime-kuma", "umami": "umami", "n8n": "n8n", "gitea": "gitea"}
	if len(list) != len(want) {
		t.Errorf("%d templates, want %d", len(list), len(want))
	}
	for _, tpl := range list {
		public, ok := want[tpl.Key]
		if !ok {
			t.Errorf("unexpected template %s", tpl.Key)
			continue
		}
		if tpl.Description == "" || tpl.DocsURL == "" || tpl.Category == "" || tpl.Logo == "" {
			t.Errorf("%s: description, docs_url, category and logo are required", tpl.Key)
		}
		s, err := domain.NewService(1, 1, tpl.Name, "t-"+tpl.Key, tpl.Compose, "localhost", Generate)
		if err != nil {
			t.Errorf("%s: %v", tpl.Key, err)
			continue
		}
		var publics []string
		for _, c := range s.Components {
			if c.Public {
				publics = append(publics, c.Name)
			}
		}
		if len(publics) != 1 || publics[0] != public {
			t.Errorf("%s: public components %v, want [%s]", tpl.Key, publics, public)
		}
		resolved, err := s.Resolved()
		if err != nil {
			t.Errorf("%s: %v", tpl.Key, err)
			continue
		}
		for _, c := range resolved.Components {
			for k, v := range c.Environment {
				if v == "" {
					t.Errorf("%s: %s.%s resolves to nothing", tpl.Key, c.Name, k)
				}
			}
		}
	}
}

func TestParseTemplateRefusals(t *testing.T) {
	for name, src := range map[string]string{
		"no name":     "compose: |\n  services:\n    a:\n      image: x\n",
		"bad compose": "name: x\ncompose: |\n  services:\n    a:\n      build: .\n",
		"not yaml":    "name: [\n",
	} {
		if _, err := domain.ParseTemplate("k", []byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := domain.ParseTemplate("Bad Key", []byte("name: x\ncompose: |\n  services:\n    a:\n      image: x\n")); err == nil {
		t.Error("bad key accepted")
	}
}
