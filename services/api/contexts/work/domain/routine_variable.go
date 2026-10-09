package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// VariableType is what kind of value a Routine variable takes.
type VariableType string

const (
	TextVariable     VariableType = "text"
	TextareaVariable VariableType = "textarea"
	NumberVariable   VariableType = "number"
	BooleanVariable  VariableType = "boolean"
	SelectVariable   VariableType = "select"
	DateVariable     VariableType = "date"
)

// VariableTypes lists every type of Routine variable, the default first.
var VariableTypes = []VariableType{TextVariable, TextareaVariable, NumberVariable, BooleanVariable, SelectVariable, DateVariable}

// ParseVariableType reads a type of Routine variable by its wire key.
func ParseVariableType(s string) (VariableType, error) {
	if t := VariableType(s); slices.Contains(VariableTypes, t) {
		return t, nil
	}
	return "", invalid("variables", "type must be text, textarea, number, boolean, select or date")
}

// BuiltinVariables are the placeholders every Routine run fills in by
// itself; they are never a Routine variable.
var BuiltinVariables = []string{"date", "timestamp"}

// RoutineVariable is a {{name}} placeholder in a Routine's title or
// description and what value it takes. Default is nil for none, else a
// value of its Type; Options are a select one's only. Label is "" for
// none.
type RoutineVariable struct {
	Name     string
	Label    string
	Type     VariableType
	Default  any
	Required bool
	Options  []string
}

// variableMatcher is Paperclip's: a markdown editor writes `_` between
// word characters as `\_`, so {{pr\_url}} is the placeholder pr_url.
var variableMatcher = regexp.MustCompile(`\{\{\s*([A-Za-z](?:\\_|[A-Za-z0-9_])*)\s*\}\}`)

var variableName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// VariableNames lists the placeholder names in the templates, in the
// order first seen.
func VariableNames(templates ...string) []string {
	var names []string
	for _, t := range templates {
		for _, m := range variableMatcher.FindAllStringSubmatch(t, -1) {
			if name := strings.ReplaceAll(m[1], `\_`, "_"); !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}

// SyncVariables is one Routine variable per placeholder in the title and
// description that is not a built-in one: the existing definition of that
// name, else a required text one without a default (date when the name
// ends in Date), as Paperclip's syncRoutineVariablesWithTemplate.
func SyncVariables(title, description string, existing []RoutineVariable) []RoutineVariable {
	out := []RoutineVariable{}
	for _, name := range VariableNames(title, description) {
		if slices.Contains(BuiltinVariables, name) {
			continue
		}
		if n := slices.IndexFunc(existing, func(v RoutineVariable) bool { return v.Name == name }); n >= 0 {
			out = append(out, existing[n])
			continue
		}
		v := RoutineVariable{Name: name, Type: TextVariable, Required: true, Options: []string{}}
		if len(name) > len("Date") && strings.HasSuffix(name, "Date") {
			v.Type = DateVariable
		}
		out = append(out, v)
	}
	return out
}

// Normalize reads raw as a value of the variable's type, as Paperclip's
// normalizeRoutineVariableValue: nil stays nil, a boolean from a boolean,
// 0/1 or words such as "yes", a number from a number or a numeric string,
// a date as YYYY-MM-DD, a select one of its options, and text as a string.
func (v RoutineVariable) Normalize(raw any) (any, error) {
	field := "variables." + v.Name
	if raw == nil {
		return nil, nil
	}
	switch v.Type {
	case BooleanVariable:
		switch b := raw.(type) {
		case bool:
			return b, nil
		case float64:
			if b == 0 || b == 1 {
				return b == 1, nil
			}
		case string:
			switch strings.ToLower(strings.TrimSpace(b)) {
			case "true", "1", "yes", "y", "on":
				return true, nil
			case "false", "0", "no", "n", "off":
				return false, nil
			}
		}
		return nil, invalid(field, "Variable %q must be a boolean", v.Name)
	case NumberVariable:
		switch n := raw.(type) {
		case float64:
			if !math.IsInf(n, 0) && !math.IsNaN(n) {
				return n, nil
			}
		case int:
			return float64(n), nil
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil && strings.TrimSpace(n) != "" && !math.IsInf(f, 0) && !math.IsNaN(f) {
				return f, nil
			}
		}
		return nil, invalid(field, "Variable %q must be a number", v.Name)
	case DateVariable:
		s, ok := raw.(string)
		if !ok {
			return nil, invalid(field, "Variable %q must be a YYYY-MM-DD date", v.Name)
		}
		s = strings.TrimSpace(s)
		if _, err := time.Parse(time.DateOnly, s); err != nil || len(s) != len(time.DateOnly) {
			return nil, invalid(field, "Variable %q must be a valid YYYY-MM-DD date", v.Name)
		}
		return s, nil
	}
	s := StringifyVariable(raw)
	if v.Type == SelectVariable && !slices.Contains(v.Options, s) {
		return nil, invalid(field, "Variable %q must match one of: %s", v.Name, strings.Join(v.Options, ", "))
	}
	return s, nil
}

// StringifyVariable writes a value as it reads in a title or description,
// as Paperclip's stringifyRoutineVariableValue: a number as JavaScript
// writes it, anything else that is not a string or boolean as JSON.
func StringifyVariable(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// missing is a value that counts as not given: none, or blank text.
func missing(v any) bool {
	s, ok := v.(string)
	return v == nil || ok && strings.TrimSpace(s) == ""
}

// check refuses a definition with an invalid name, a type it does not
// know, a select one without options, or a default not of its type, and
// answers it with its default normalized.
func (v RoutineVariable) check() (RoutineVariable, error) {
	if !variableName.MatchString(v.Name) {
		return v, invalid("variables", "%q is not a variable name: a letter, then letters, digits or _", v.Name)
	}
	if _, err := ParseVariableType(string(v.Type)); err != nil {
		return v, invalid("variables."+v.Name, "type must be text, textarea, number, boolean, select or date")
	}
	if v.Options == nil {
		v.Options = []string{}
	}
	if v.Type == SelectVariable && len(v.Options) == 0 {
		return v, invalid("variables."+v.Name, "Variable %q must define at least one option", v.Name)
	}
	if v.Type != SelectVariable {
		v.Options = []string{}
	}
	d, err := v.Normalize(v.Default)
	if err != nil {
		return v, err
	}
	v.Default = d
	v.Label = strings.TrimSpace(v.Label)
	return v, nil
}

// SetVariables replaces the Routine variables' definitions with defs,
// kept in step with the placeholders: a definition of a name that is not
// one is dropped, and a placeholder without one gets the default one.
func (r *Routine) SetVariables(defs []RoutineVariable) error {
	seen := map[string]bool{}
	for _, d := range defs {
		if seen[d.Name] {
			return invalid("variables", "variable %q is defined twice", d.Name)
		}
		seen[d.Name] = true
	}
	vars := SyncVariables(r.Title, r.Description, defs)
	for n, v := range vars {
		c, err := v.check()
		if err != nil {
			return err
		}
		vars[n] = c
	}
	r.Variables = vars
	return nil
}

// RequiredWithoutDefault names the required Routine variables without a
// default: a Schedule, which gives no values, could not fill them.
func (r Routine) RequiredWithoutDefault() []string {
	var names []string
	for _, v := range r.Variables {
		if d, err := v.Normalize(v.Default); v.Required && (err != nil || missing(d)) {
			names = append(names, v.Name)
		}
	}
	return names
}

// CheckSchedulable refuses a Routine that a Schedule trigger could not
// run, as Paperclip's assertScheduleCompatibleVariables.
func (r Routine) CheckSchedulable(field string) error {
	if names := r.RequiredWithoutDefault(); len(names) > 0 {
		return invalid(field, "Scheduled routines require defaults for required variables: %s", strings.Join(names, ", "))
	}
	return nil
}

// BuiltinValues are the values of the built-in variables at now, in UTC,
// as Paperclip's: date as YYYY-MM-DD, timestamp as
// "April 28, 2026 at 12:17 PM UTC".
func BuiltinValues(now time.Time) map[string]any {
	now = now.UTC()
	return map[string]any{
		"date":      now.Format(time.DateOnly),
		"timestamp": now.Format("January 2, 2006 at 3:04 PM") + " UTC",
	}
}

// ResolveVariables is the value of each Routine variable for a Routine
// run from source, as Paperclip's resolveRoutineVariableValues, plus the
// built-in ones at now. A value is the one given, else the payload's
// variables object's, else (for a webhook) the payload's own field of
// that name, else the default. A required variable still missing refuses
// the Routine run, naming it.
func ResolveVariables(vars []RoutineVariable, source RoutineRunSource, payload, given map[string]any, now time.Time) (map[string]any, error) {
	provided := map[string]any{}
	if source == WebhookSource {
		for k, v := range payload {
			provided[k] = v
		}
	}
	if nested, ok := payload["variables"].(map[string]any); ok {
		for k, v := range nested {
			provided[k] = v
		}
	}
	for k, v := range given {
		provided[k] = v
	}
	delete(provided, "variables")
	values := BuiltinValues(now)
	var absent []string
	for _, v := range vars {
		raw, ok := provided[v.Name]
		if !ok {
			raw = v.Default
		}
		value, err := v.Normalize(raw)
		if err != nil {
			return nil, err
		}
		if missing(value) {
			if v.Required {
				absent = append(absent, v.Name)
			}
			continue
		}
		values[v.Name] = value
	}
	if len(absent) > 0 {
		return nil, invalid("variables."+absent[0], "Missing routine variables: %s", strings.Join(absent, ", "))
	}
	return values, nil
}

// Interpolate replaces each placeholder in the template that values has
// with its value, and leaves any other as written.
func Interpolate(template string, values map[string]any) string {
	if len(values) == 0 {
		return template
	}
	return variableMatcher.ReplaceAllStringFunc(template, func(m string) string {
		name := strings.ReplaceAll(variableMatcher.FindStringSubmatch(m)[1], `\_`, "_")
		if v, ok := values[name]; ok {
			return StringifyVariable(v)
		}
		return m
	})
}
