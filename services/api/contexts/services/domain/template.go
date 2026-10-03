package domain

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Template is a Service template: a named, described Compose file from the
// catalog. Category and Logo are Coolify's template headers: the New
// Resource page filters on the first and shows the second, a file under the
// dashboard's svgs/.
type Template struct {
	Key         string
	Name        string
	Description string
	DocsURL     string
	Category    string
	Logo        string
	Tags        []string
	Compose     string
}

var templateKey = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ParseTemplate reads one template file; key is its file name without
// .yaml. The Compose file must parse like any other.
func ParseTemplate(key string, src []byte) (Template, error) {
	var raw struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		DocsURL     string   `yaml:"docs_url"`
		Category    string   `yaml:"category"`
		Logo        string   `yaml:"logo"`
		Tags        []string `yaml:"tags"`
		Compose     string   `yaml:"compose"`
	}
	if err := yaml.Unmarshal(src, &raw); err != nil {
		return Template{}, fmt.Errorf("template %s: %w", key, err)
	}
	t := Template{Key: key, Name: strings.TrimSpace(raw.Name), Description: strings.TrimSpace(raw.Description), DocsURL: raw.DocsURL, Category: strings.TrimSpace(raw.Category), Logo: strings.TrimSpace(raw.Logo), Tags: raw.Tags, Compose: raw.Compose}
	switch {
	case !templateKey.MatchString(key):
		return Template{}, fmt.Errorf("template %q: the key is lowercase letters, digits and -", key)
	case t.Name == "" || t.Compose == "":
		return Template{}, fmt.Errorf("template %s: name and compose are required", key)
	}
	if _, err := ParseCompose(t.Compose); err != nil {
		return Template{}, fmt.Errorf("template %s: %w", key, err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t, nil
}
