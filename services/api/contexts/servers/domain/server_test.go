package domain

import (
	"errors"
	"testing"
)

var key = ServerKey{Public: "ssh-ed25519 AAAA bakery@x", Private: "-----BEGIN OPENSSH PRIVATE KEY-----"}

func TestNewRemote(t *testing.T) {
	s, err := NewRemote(Input{Name: " web 1 ", Host: "203.0.113.10", User: "bakery"}, key)
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
		if _, err := NewRemote(in, key); !errors.As(err, &fe) {
			t.Errorf("%+v: err %v, want a FieldError", in, err)
		}
	}
	if _, err := NewRemote(Input{Name: "a", Host: "h", User: "u"}, ServerKey{}); err == nil {
		t.Error("no key accepted")
	}
}

func TestLocalServerIsFixed(t *testing.T) {
	s := NewLocal()
	if err := s.Edit(Input{Name: "x", Host: "h", User: "u"}); !errors.Is(err, ErrLocalServer) {
		t.Errorf("edit: %v", err)
	}
	if err := s.CanDelete(); !errors.Is(err, ErrLocalServer) {
		t.Errorf("delete: %v", err)
	}
	if err := s.ForgetHostKey(); !errors.Is(err, ErrLocalServer) {
		t.Errorf("forget: %v", err)
	}
}

func TestEditAddressForgetsHostKey(t *testing.T) {
	s, _ := NewRemote(Input{Name: "a", Host: "h", User: "u"}, key)
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
	s, _ := NewRemote(Input{Name: "a", Host: "h", User: "u"}, key)
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
