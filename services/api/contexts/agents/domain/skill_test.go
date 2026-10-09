package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func fieldOf(err error) string {
	var fe *FieldError
	if errors.As(err, &fe) {
		return fe.Field
	}
	return ""
}

func TestNormalizeSkillSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Release notes":                "release-notes",
		"  --Hello, World!-- ":         "hello-world",
		"Ünïcode 2":                    "n-code-2",
		"!!!":                          "",
		strings.Repeat("ab", 40):       strings.Repeat("ab", 32),
		strings.Repeat("a", 63) + " b": strings.Repeat("a", 63),
	} {
		if got := NormalizeSkillSlug(in); got != want {
			t.Errorf("NormalizeSkillSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewSkill(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s, err := NewSkill(1, "Release notes", "", "Write release notes", "", 7, at)
	if err != nil {
		t.Fatal(err)
	}
	if s.Slug != "release-notes" || s.Name != "Release notes" || s.Description != "Write release notes" {
		t.Fatalf("got %q %q %q", s.Slug, s.Name, s.Description)
	}
	md := string(s.Files[SkillMarkdown].Content)
	if !strings.HasPrefix(md, "---\nname: Release notes\ndescription: Write release notes\n---\n\n# Release notes\n") {
		t.Fatalf("fallback SKILL.md:\n%s", md)
	}
	bare, _ := NewSkill(1, "Bare", "", "", "", 7, at)
	if got := string(bare.Files[SkillMarkdown].Content); !strings.Contains(got, "Describe what this skill does.") || strings.Contains(got, "description:") {
		t.Fatalf("fallback without description:\n%s", got)
	}
	given, err := NewSkill(1, "Given", "given", "", "---\nname: \"From md\"\ndescription: 'Says so'\n---\nBody", 7, at)
	if err != nil || given.Name != "From md" || given.Description != "Says so" {
		t.Fatalf("markdown's frontmatter: %+v %v", given, err)
	}
	for name, c := range map[string]struct{ name, slug, field string }{
		"no name":      {"", "", "name"},
		"bakery":       {"x", "bakery", "slug"},
		"bakery named": {"Bakery", "", "slug"},
		"bad slug":     {"x", "Not OK", "slug"},
		"long slug":    {"x", strings.Repeat("a", 65), "slug"},
		"no slug":      {"!!!", "", "slug"},
		"long name":    {strings.Repeat("n", 101), "", "name"},
	} {
		if _, err := NewSkill(1, c.name, c.slug, "", "", 7, at); fieldOf(err) != c.field {
			t.Errorf("%s: %v, want a %s error", name, err, c.field)
		}
	}
}

func TestSkillFiles(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s, _ := NewSkill(1, "Notes", "", "", "", 7, at)
	changed, err := s.WriteFile("templates/notes.md", []byte("# Notes"), false, at)
	if err != nil || !changed {
		t.Fatalf("write: %v %v", changed, err)
	}
	if changed, _ := s.WriteFile("templates/notes.md", []byte("# Notes"), false, at); changed {
		t.Fatal("the same file again changed")
	}
	if changed, _ := s.WriteFile("templates/notes.md", []byte("# Notes"), true, at); !changed {
		t.Fatal("making it executable changed nothing")
	}
	for _, p := range []string{"", "../x", "a/../x", "/x", "a//b", "a\\b", "./a", "a/", strings.Repeat("p", 513)} {
		if _, err := s.WriteFile(p, []byte("x"), false, at); fieldOf(err) != "path" {
			t.Errorf("path %q: %v", p, err)
		}
	}
	if _, err := s.WriteFile("big", make([]byte, MaxSkillFileSize+1), false, at); fieldOf(err) != "content" {
		t.Errorf("a file over 512 KiB: %v", err)
	}
	for i := range 3 {
		if _, err := s.WriteFile("blob"+string(rune('a'+i)), make([]byte, MaxSkillFileSize), false, at); err != nil {
			t.Fatalf("blob %d: %v", i, err)
		}
	}
	if _, err := s.WriteFile("one-more", make([]byte, MaxSkillFileSize), false, at); fieldOf(err) != "content" {
		t.Errorf("a skill over 2 MiB: %v", err)
	}
	if _, err := s.WriteFile(SkillMarkdown, []byte("---\nname: Renamed\n---\n"), false, at); err != nil || s.Name != "Renamed" || s.Description != "" {
		t.Fatalf("rewriting SKILL.md: %q %q %v", s.Name, s.Description, err)
	}
	if _, err := s.WriteFile(SkillMarkdown, []byte("no frontmatter"), false, at); err != nil || s.Name != "notes" {
		t.Fatalf("SKILL.md without a name: %q %v", s.Name, err)
	}
	if _, err := s.WriteFile(SkillMarkdown, []byte("---\nname: "+strings.Repeat("n", 101)+"\n---\n"), false, at); fieldOf(err) != "content" {
		t.Errorf("a name over 100: %v", err)
	}
	if _, err := s.WriteFile(SkillMarkdown, []byte{0xff, 0xfe}, false, at); fieldOf(err) != "content" {
		t.Errorf("SKILL.md not UTF-8: %v", err)
	}
	if err := s.DeleteFile(SkillMarkdown, at); fieldOf(err) != "path" {
		t.Errorf("deleting SKILL.md: %v", err)
	}
	if err := s.DeleteFile("templates/notes.md", at); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile("templates/notes.md", at); fieldOf(err) != "path" {
		t.Errorf("deleting a file it lacks: %v", err)
	}
}

func TestSkillFileLimit(t *testing.T) {
	at := time.Now()
	s, _ := NewSkill(1, "Many", "", "", "", 7, at)
	for i := 1; i < MaxSkillFiles; i++ {
		if _, err := s.WriteFile("f/"+strings.Repeat("x", i), nil, false, at); err != nil {
			t.Fatalf("file %d: %v", i, err)
		}
	}
	if _, err := s.WriteFile("f/over", nil, false, at); fieldOf(err) != "path" {
		t.Errorf("file %d: %v", MaxSkillFiles+1, err)
	}
}

func TestSkillContent(t *testing.T) {
	if c, e := EncodeSkillContent([]byte("hé")); c != "hé" || e != UTF8Encoding {
		t.Errorf("text: %q %q", c, e)
	}
	if c, e := EncodeSkillContent([]byte{0, 1}); c != "AAE=" || e != Base64Encoding {
		t.Errorf("binary: %q %q", c, e)
	}
	if b, err := DecodeSkillContent("AAE=", Base64Encoding); err != nil || len(b) != 2 {
		t.Errorf("base64: %v %v", b, err)
	}
	if _, err := DecodeSkillContent("!!", Base64Encoding); fieldOf(err) != "content" {
		t.Errorf("bad base64: %v", err)
	}
	if _, err := DecodeSkillContent("x", "hex"); fieldOf(err) != "encoding" {
		t.Errorf("bad encoding: %v", err)
	}
	for p, want := range map[string]string{"SKILL.md": "skill", "a/b.MD": "markdown", "run.sh": "other"} {
		if got := SkillFileKind(p, false); got != want {
			t.Errorf("kind of %s = %s", p, got)
		}
	}
	if SkillFileKind("run.sh", true) != "script" {
		t.Error("an executable is not a script")
	}
}
