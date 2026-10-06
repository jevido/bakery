package domain

import (
	"errors"
	"strings"
	"testing"
)

var key = PrivateKey{Public: "ssh-ed25519 AAAA bakery@x", Private: "-----BEGIN OPENSSH PRIVATE KEY-----"}

func TestNewRemote(t *testing.T) {
	s, err := NewRemote(1, Input{Name: " web 1 ", Host: "203.0.113.10", User: "bakery"}, key)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "web 1" || s.Port != DefaultPort || s.Kind != Remote || s.Status != Unvalidated {
		t.Fatalf("%+v", s)
	}
	for _, in := range []Input{
		{Name: "", Host: "h", User: "u"},
		{Name: "localhost", Host: "h", User: "u"},
		{Name: "a", Host: "", User: "u"},
		{Name: "a", Host: "bad host", User: "u"},
		{Name: "a", Host: "h", Port: 70000, User: "u"},
		{Name: "a", Host: "h", User: "Root!"},
	} {
		var fe *FieldError
		if _, err := NewRemote(1, in, key); !errors.As(err, &fe) {
			t.Errorf("%+v: err %v, want a FieldError", in, err)
		}
	}
	if _, err := NewRemote(1, Input{Name: "a", Host: "h", User: "u"}, PrivateKey{}); err == nil {
		t.Error("no key accepted")
	}
}

func TestLocalServerIsFixed(t *testing.T) {
	s := NewLocal()
	for _, in := range []Input{
		{Name: "x", Host: "h", User: "u"},
		{Name: "x", Port: 22},
		{Name: "x", User: "u"},
	} {
		if err := s.Edit(in); !errors.Is(err, ErrLocalServer) {
			t.Errorf("edit %+v: %v", in, err)
		}
	}
	if s.Name != LocalName {
		t.Errorf("a refused edit renamed it to %q", s.Name)
	}
	if err := s.CanDelete(); !errors.Is(err, ErrLocalServer) {
		t.Errorf("delete: %v", err)
	}
	if err := s.ForgetHostKey(); !errors.Is(err, ErrLocalServer) {
		t.Errorf("forget: %v", err)
	}
}

func TestEditLocalNameAndDescription(t *testing.T) {
	s := NewLocal()
	if err := s.Edit(Input{Name: " main box ", Description: "  the one we run on  "}); err != nil {
		t.Fatal(err)
	}
	if s.Name != "main box" || s.Description != "the one we run on" || s.Kind != Local {
		t.Fatalf("%+v", s)
	}
	var fe *FieldError
	if err := s.Edit(Input{Name: ""}); !errors.As(err, &fe) || fe.Field != "name" {
		t.Errorf("empty name: %v", err)
	}
	if err := s.Edit(Input{Name: "x", Description: strings.Repeat("é", 256)}); !errors.As(err, &fe) || fe.Field != "description" {
		t.Errorf("long description: %v", err)
	}
	if s.Name != "main box" {
		t.Error("a refused edit changed the Server")
	}
}

func TestDescription(t *testing.T) {
	s, err := NewRemote(1, Input{Name: "a", Description: "  web  ", Host: "h", User: "u"}, key)
	if err != nil || s.Description != "web" {
		t.Fatalf("new: %v, %q", err, s.Description)
	}
	if err := s.Edit(Input{Name: "a", Description: strings.Repeat("é", 255), Host: "h", User: "u"}); err != nil {
		t.Fatal(err)
	}
	var fe *FieldError
	if _, err := NewRemote(1, Input{Name: "a", Description: strings.Repeat("x", 256), Host: "h", User: "u"}, key); !errors.As(err, &fe) || fe.Field != "description" {
		t.Errorf("long description: %v", err)
	}
	if err := s.Edit(Input{Name: "a", Host: "h", User: "u"}); err != nil || s.Description != "" {
		t.Fatalf("clear: %v, %q", err, s.Description)
	}
}

func TestEditAddressForgetsHostKey(t *testing.T) {
	s, _ := NewRemote(1, Input{Name: "a", Host: "h", User: "u"}, key)
	s.RecordValidation(Validation{Checks: []Check{{Name: CheckSSH, OK: true, Required: true}}}, "ssh-ed25519 HOST")
	if s.HostKey == "" || s.Status != Reachable {
		t.Fatalf("%+v", s)
	}
	if err := s.Edit(Input{Name: "b", Host: "h", User: "u"}); err != nil {
		t.Fatal(err)
	}
	if s.HostKey == "" || s.Status != Reachable {
		t.Fatal("renaming dropped the host key")
	}
	if err := s.Edit(Input{Name: "b", Host: "h2", User: "u"}); err != nil {
		t.Fatal(err)
	}
	if s.HostKey != "" || s.Status != Unvalidated {
		t.Fatalf("new host kept %+v", s)
	}
}

func TestRecordValidation(t *testing.T) {
	s, _ := NewRemote(1, Input{Name: "a", Host: "h", User: "u"}, key)
	s.RecordValidation(Validation{Checks: []Check{
		{Name: CheckSSH, OK: true, Required: true},
		{Name: CheckPorts, OK: false},
	}}, "ssh-ed25519 ONE")
	if s.Status != Reachable || s.HostKey != "ssh-ed25519 ONE" {
		t.Fatalf("%+v", s)
	}
	// A pinned key is never replaced by a validation.
	s.RecordValidation(Validation{Checks: []Check{{Name: CheckLinger, OK: false, Required: true}}}, "ssh-ed25519 TWO")
	if s.Status != Unreachable || s.HostKey != "ssh-ed25519 ONE" {
		t.Fatalf("%+v", s)
	}
	if (Validation{}).Passed() {
		t.Error("an empty validation passed")
	}
}
