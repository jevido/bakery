// Package templates is the catalog of Service templates, embedded in the
// API: one YAML file per template, named after its key.
package templates

import "embed"

//go:embed *.yaml
var Files embed.FS
