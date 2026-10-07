// Package domain is the guilds model: Guilds, the Roles Members hold in
// them, and the rules for both.
package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxNameLength and MaxDescriptionLength are in characters.
const (
	MaxNameLength        = 255
	MaxDescriptionLength = 255
)

var (
	ErrInvalidName        = errors.New("name is required and at most 255 characters")
	ErrDescriptionTooLong = errors.New("description is at most 255 characters")
)

// Guild is a group of Members that owns Projects, Servers, S3 storages,
// Notification channels, Known hosts and API tokens.
type Guild struct {
	ID          uint64
	Name        string
	Description string
	// MasterID is the Member who is the Guild Master: above every Role,
	// with every Permission, changed only by an accepted Offer.
	MasterID uint64
}

// FirstGuildName is what Setup and the migration from before Guilds call
// the installation's first Guild.
const FirstGuildName = "Default"

// NewGuild validates a new Guild, whose creator is its Guild Master, and
// returns it without an ID.
func NewGuild(name, description string, creatorID uint64) (Guild, error) {
	g := Guild{MasterID: creatorID}
	if err := g.Rename(name); err != nil {
		return Guild{}, err
	}
	if err := g.ChangeDescription(description); err != nil {
		return Guild{}, err
	}
	return g, nil
}

// Rename trims name and refuses an empty or too long one.
func (g *Guild) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return ErrInvalidName
	}
	g.Name = name
	return nil
}

// ChangeDescription trims description; empty is allowed.
func (g *Guild) ChangeDescription(description string) error {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		return ErrDescriptionTooLong
	}
	g.Description = description
	return nil
}
