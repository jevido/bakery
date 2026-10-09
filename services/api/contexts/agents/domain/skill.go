package domain

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// SkillMarkdown is the one Skill file every Skill has, and what
	// `claude` reads its name and description from.
	SkillMarkdown = "SKILL.md"
	// BakerySlug is The Bakery skill's, which the Desktop writes beside
	// every Skill, so no Skill may take it.
	BakerySlug = "bakery"

	MaxSkillSlug        = 64
	MaxSkillName        = 100
	MaxSkillDescription = 1000
	MaxSkillPath        = 512
	MaxSkillFileSize    = 512 << 10
	MaxSkillSize        = 2 << 20
	MaxSkillFiles       = 200
)

// ErrSkillSlugTaken refuses a Skill slug another Skill of the Guild has.
var ErrSkillSlugTaken = errors.New("skill slug taken")

// Skill file encodings on the wire: utf8 text, or base64 for any other
// bytes.
const (
	UTF8Encoding   = "utf8"
	Base64Encoding = "base64"
)

// SkillFile is one file of a Skill: its bytes and whether it is executable.
type SkillFile struct {
	Content    []byte
	Executable bool
	UpdatedAt  time.Time
}

// Skill is a Claude skill package a Guild keeps: a SKILL.md and its other
// Skill files, by path. Its Name and Description come from SKILL.md's
// frontmatter only.
type Skill struct {
	ID          uint64
	GuildID     uint64
	Slug        string
	Name        string
	Description string
	Files       map[string]SkillFile
	CreatedBy   uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NormalizeSkillSlug is Paperclip's normalizeSkillSlug: lowercased, every
// run of characters other than a–z and 0–9 one `-`, trimmed of `-` and to
// MaxSkillSlug.
func NormalizeSkillSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	out := b.String()
	if len(out) > MaxSkillSlug {
		out = strings.TrimRight(out[:MaxSkillSlug], "-")
	}
	return out
}

// CheckSkillSlug refuses a Skill slug that is not 1–64 lowercase letters,
// digits and `-`, or is The Bakery skill's.
func CheckSkillSlug(slug string) error {
	if slug == "" {
		return invalid("slug", "is required")
	}
	if len(slug) > MaxSkillSlug {
		return invalid("slug", "must be at most %d characters", MaxSkillSlug)
	}
	for _, r := range slug {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return invalid("slug", "may only hold lowercase letters, digits and -")
		}
	}
	if slug == BakerySlug {
		return invalid("slug", "is The Bakery skill's")
	}
	return nil
}

// FallbackSkillMarkdown is Paperclip's SKILL.md for a Skill created
// without one: a frontmatter naming it, its heading and its description.
func FallbackSkillMarkdown(name, description string) string {
	lines := []string{"---", "name: " + name}
	if description != "" {
		lines = append(lines, "description: "+description)
	}
	if description == "" {
		description = "Describe what this skill does."
	}
	return strings.Join(append(lines, "---", "", "# "+name, "", description, ""), "\n")
}

// NewSkill is a Skill of the Guild named name, its Skill slug the given one
// or derived from the name. Its SKILL.md is markdown, or the fallback for
// the name and description when markdown is blank.
func NewSkill(guildID uint64, name, slug, description, markdown string, by uint64, at time.Time) (Skill, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if name == "" {
		return Skill{}, invalid("name", "is required")
	}
	if utf8.RuneCountInString(name) > MaxSkillName {
		return Skill{}, invalid("name", "must be at most %d characters", MaxSkillName)
	}
	if utf8.RuneCountInString(description) > MaxSkillDescription {
		return Skill{}, invalid("description", "must be at most %d characters", MaxSkillDescription)
	}
	if slug = strings.TrimSpace(slug); slug == "" {
		if slug = NormalizeSkillSlug(name); slug == "" {
			return Skill{}, invalid("slug", "cannot be derived from the name; give one")
		}
	}
	if err := CheckSkillSlug(slug); err != nil {
		return Skill{}, err
	}
	if strings.TrimSpace(markdown) == "" {
		markdown = FallbackSkillMarkdown(name, description)
	}
	s := Skill{GuildID: guildID, Slug: slug, Files: map[string]SkillFile{}, CreatedBy: by, CreatedAt: at}
	if _, err := s.WriteFile(SkillMarkdown, []byte(markdown), false, at); err != nil {
		if fe := (*FieldError)(nil); errors.As(err, &fe) && fe.Field == "content" {
			return Skill{}, invalid("markdown", "%s", fe.Message)
		}
		return Skill{}, err
	}
	return s, nil
}

// CheckSkillPath refuses a Skill file path that is not relative, holds
// `..`, `.`, `\` or an empty segment, or is longer than MaxSkillPath.
func CheckSkillPath(p string) error {
	switch {
	case p == "":
		return invalid("path", "is required")
	case len(p) > MaxSkillPath:
		return invalid("path", "must be at most %d characters", MaxSkillPath)
	case strings.HasPrefix(p, "/"):
		return invalid("path", "must be relative")
	case strings.ContainsAny(p, "\\\x00"):
		return invalid("path", "may not hold \\")
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "":
			return invalid("path", "may not hold an empty segment")
		case ".", "..":
			return invalid("path", "may not hold . or ..")
		}
	}
	return nil
}

// DecodeSkillContent reads a Skill file's content as sent in its encoding,
// utf8 when none is given.
func DecodeSkillContent(content, encoding string) ([]byte, error) {
	switch encoding {
	case "", UTF8Encoding:
		return []byte(content), nil
	case Base64Encoding:
		b, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, invalid("content", "is not valid base64")
		}
		return b, nil
	}
	return nil, invalid("encoding", "must be utf8 or base64")
}

// EncodeSkillContent is a Skill file's content for the wire: utf8 when the
// bytes are valid UTF-8 without NUL, base64 otherwise.
func EncodeSkillContent(b []byte) (content, encoding string) {
	if utf8.Valid(b) && bytes.IndexByte(b, 0) < 0 {
		return string(b), UTF8Encoding
	}
	return base64.StdEncoding.EncodeToString(b), Base64Encoding
}

// SkillFileKind is what the dashboard shows a Skill file as, by its path:
// skill for SKILL.md, markdown for another .md file, script for an
// executable one and other otherwise.
func SkillFileKind(p string, executable bool) string {
	switch {
	case p == SkillMarkdown:
		return "skill"
	case strings.EqualFold(path.Ext(p), ".md"):
		return "markdown"
	case executable:
		return "script"
	}
	return "other"
}

// Size is the bytes of all the Skill's files.
func (s Skill) Size() int {
	n := 0
	for _, f := range s.Files {
		n += len(f.Content)
	}
	return n
}

// WriteFile creates or replaces the Skill file at p; writing SKILL.md reads
// the Skill's name and description from it again. changed is false when
// the file was already exactly this.
func (s *Skill) WriteFile(p string, content []byte, executable bool, at time.Time) (changed bool, err error) {
	if err := CheckSkillPath(p); err != nil {
		return false, err
	}
	if len(content) > MaxSkillFileSize {
		return false, invalid("content", "must be at most %d KiB", MaxSkillFileSize>>10)
	}
	old, had := s.Files[p]
	if had && old.Executable == executable && bytes.Equal(old.Content, content) {
		return false, nil
	}
	if !had && len(s.Files) >= MaxSkillFiles {
		return false, invalid("path", "a skill holds at most %d files", MaxSkillFiles)
	}
	if s.Size()-len(old.Content)+len(content) > MaxSkillSize {
		return false, invalid("content", "a skill holds at most %d MiB", MaxSkillSize>>20)
	}
	name, description := s.Name, s.Description
	if p == SkillMarkdown {
		if !utf8.Valid(content) {
			return false, invalid("content", "SKILL.md must be UTF-8 text")
		}
		if name, description, err = frontmatter(string(content), s.Slug); err != nil {
			return false, err
		}
	}
	s.Files[p] = SkillFile{Content: bytes.Clone(content), Executable: executable, UpdatedAt: at}
	s.Name, s.Description, s.UpdatedAt = name, description, at
	return true, nil
}

// DeleteFile removes the Skill file at p, which is never SKILL.md.
func (s *Skill) DeleteFile(p string, at time.Time) error {
	if p == SkillMarkdown {
		return invalid("path", "SKILL.md cannot be deleted")
	}
	if _, ok := s.Files[p]; !ok {
		return invalid("path", "the skill has no such file")
	}
	delete(s.Files, p)
	s.UpdatedAt = at
	return nil
}

// frontmatter reads name and description from the `---` fenced block at
// the top of a SKILL.md: `key: value` lines, one level only, quotes
// stripped. A missing name is the Skill slug.
func frontmatter(md, slug string) (name, description string, err error) {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for _, l := range lines[1:] {
			if strings.TrimSpace(l) == "---" {
				break
			}
			key, value, ok := strings.Cut(l, ":")
			if !ok || strings.HasPrefix(key, " ") || strings.HasPrefix(key, "\t") {
				continue
			}
			value = unquote(strings.TrimSpace(value))
			switch strings.TrimSpace(key) {
			case "name":
				name = value
			case "description":
				description = value
			}
		}
	}
	if name == "" {
		name = slug
	}
	if utf8.RuneCountInString(name) > MaxSkillName {
		return "", "", invalid("content", "the frontmatter's name must be at most %d characters", MaxSkillName)
	}
	if utf8.RuneCountInString(description) > MaxSkillDescription {
		return "", "", invalid("content", "the frontmatter's description must be at most %d characters", MaxSkillDescription)
	}
	return name, description, nil
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		return strings.TrimSpace(v[1 : len(v)-1])
	}
	return v
}

// SkillCreated is published when a Member has created a Skill.
type SkillCreated struct {
	Skill   Skill
	ActorID uint64
}

// SkillFileUpdated is published when a Member has written a Skill file
// that changed.
type SkillFileUpdated struct {
	Skill   Skill
	ActorID uint64
	Path    string
}

// SkillFileDeleted is published when a Member has deleted a Skill file.
type SkillFileDeleted struct {
	Skill   Skill
	ActorID uint64
	Path    string
}

// SkillDeleted is published when a Member has deleted a Skill.
type SkillDeleted struct {
	Skill   Skill
	ActorID uint64
}
