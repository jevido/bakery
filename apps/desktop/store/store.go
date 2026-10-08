// Package store keeps the Bakeries this Desktop app is connected to, with
// their Desktop keys, in one JSON file only its person may read (mode 0600),
// as Paperclip's CLI keeps auth.json. The OS keyring was passed over: it adds
// a dependency and a D-Bus secret service the headless checks do not have.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Member is the person a Bakery's Desktop key acts as.
type Member struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Bakery is one connected Bakery.
type Bakery struct {
	Address     string    `json:"address"`
	DesktopID   uint64    `json:"desktop_id"`
	Key         string    `json:"key"`
	Member      Member    `json:"member"`
	ConnectedAt time.Time `json:"connected_at"`
	// SignedOut is set when the Bakery answered 401 to the key: it was
	// signed out there, and connecting again replaces it.
	SignedOut bool `json:"signed_out,omitempty"`
}

// File is bakeries.json.
type File struct {
	Version  int      `json:"version"`
	Bakeries []Bakery `json:"bakeries"`
	// Active is the address of the Bakery the window shows.
	Active string `json:"active,omitempty"`
}

// Store reads and writes bakeries.json at Path. Every change rewrites the
// whole file.
type Store struct {
	Path string
	mu   sync.Mutex
}

// DefaultPath is $BAKERY_DESKTOP_HOME/bakeries.json, or the-bakery under
// the OS's per-person config directory.
func DefaultPath() (string, error) {
	if home := os.Getenv("BAKERY_DESKTOP_HOME"); home != "" {
		return filepath.Join(home, "bakeries.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "the-bakery", "bakeries.json"), nil
}

// New returns a Store on path.
func New(path string) *Store {
	return &Store{Path: path}
}

// Load reads the file; a missing one is an empty File.
func (s *Store) Load() (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() (File, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return File{Version: 1, Bakeries: []Bakery{}}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("%s: %w", s.Path, err)
	}
	if f.Version != 1 {
		return File{}, fmt.Errorf("%s: version %d is not one this app reads", s.Path, f.Version)
	}
	if f.Bakeries == nil {
		f.Bakeries = []Bakery{}
	}
	return f, nil
}

// save writes through a temporary file created 0600 and renamed over the
// old one, so the keys are never readable by others, even for a moment, and
// a crash never leaves half a file.
func (s *Store) save(f File) error {
	f.Version = 1
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".bakeries-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path)
}

// Update loads the file, lets change edit it and saves it, under one lock.
func (s *Store) Update(change func(*File) error) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return File{}, err
	}
	if err := change(&f); err != nil {
		return File{}, err
	}
	return f, s.save(f)
}

// Put adds b, replacing a Bakery with the same address, and makes it the
// active one.
func (s *Store) Put(b Bakery) error {
	_, err := s.Update(func(f *File) error {
		f.Bakeries = without(f.Bakeries, b.Address)
		f.Bakeries = append(f.Bakeries, b)
		f.Active = b.Address
		return nil
	})
	return err
}

// Remove forgets the Bakery at address; when it was active, the first one
// left becomes active.
func (s *Store) Remove(address string) error {
	_, err := s.Update(func(f *File) error {
		f.Bakeries = without(f.Bakeries, address)
		if f.Active == address {
			f.Active = ""
			if len(f.Bakeries) > 0 {
				f.Active = f.Bakeries[0].Address
			}
		}
		return nil
	})
	return err
}

// ErrUnknown is an address no connected Bakery has.
var ErrUnknown = errors.New("no Bakery is connected at that address")

// Activate makes the Bakery at address the one the window shows.
func (s *Store) Activate(address string) error {
	_, err := s.Update(func(f *File) error {
		if _, ok := f.Find(address); !ok {
			return ErrUnknown
		}
		f.Active = address
		return nil
	})
	return err
}

// MarkSignedOut records that the Bakery at address refused its key.
func (s *Store) MarkSignedOut(address string) error {
	_, err := s.Update(func(f *File) error {
		for i := range f.Bakeries {
			if f.Bakeries[i].Address == address {
				f.Bakeries[i].SignedOut = true
			}
		}
		return nil
	})
	return err
}

// Find is the Bakery at address.
func (f File) Find(address string) (Bakery, bool) {
	for _, b := range f.Bakeries {
		if b.Address == address {
			return b, true
		}
	}
	return Bakery{}, false
}

func without(bs []Bakery, address string) []Bakery {
	out := make([]Bakery, 0, len(bs))
	for _, b := range bs {
		if b.Address != address {
			out = append(out, b)
		}
	}
	return out
}
