package infra

import (
	"context"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type knownHostRecord struct {
	ID        uint64 `gorm:"primaryKey"`
	GuildID   uint64
	Host      string
	Keys      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (knownHostRecord) TableName() string { return "known_hosts" }

// KnownHosts stores Known hosts in the known_hosts table.
type KnownHosts struct{}

func (KnownHosts) Lines(ctx context.Context, guildID uint64) (string, error) {
	var recs []knownHostRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("guild_id", guildID).OrderBy("id").Find(&recs); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, r := range recs {
		b.WriteString(strings.TrimSpace(r.Keys))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// Remember groups the lines by host and adds each line a host does not have
// yet. A host's existing keys are never replaced: that is what ForgetKnownHost
// is for.
func (k KnownHosts) Remember(ctx context.Context, guildID uint64, lines string) error {
	byHost := map[string][]string{}
	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		host, _, ok := strings.Cut(line, " ")
		if line == "" || strings.HasPrefix(line, "#") || !ok {
			continue
		}
		byHost[host] = append(byHost[host], line)
	}
	q := facades.Orm().WithContext(ctx).Query()
	for host, add := range byHost {
		var recs []knownHostRecord
		if err := q.Where("guild_id", guildID).Where("host", host).Find(&recs); err != nil {
			return err
		}
		if len(recs) == 0 {
			now := time.Now()
			if err := q.Create(&knownHostRecord{GuildID: guildID, Host: host, Keys: strings.Join(add, "\n"), CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			continue
		}
		have := strings.Split(recs[0].Keys, "\n")
		merged := append([]string{}, have...)
		for _, l := range add {
			if !contains(have, l) {
				merged = append(merged, l)
			}
		}
		if len(merged) == len(have) {
			continue
		}
		if _, err := q.Model(&knownHostRecord{}).Where("id", recs[0].ID).Update(map[string]any{
			"keys": strings.Join(merged, "\n"), "updated_at": time.Now(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (KnownHosts) List(ctx context.Context, guildID uint64) ([]domain.KnownHost, error) {
	var recs []knownHostRecord
	if err := facades.Orm().WithContext(ctx).Query().Where("guild_id", guildID).OrderBy("host").Find(&recs); err != nil {
		return nil, err
	}
	out := make([]domain.KnownHost, len(recs))
	for i, r := range recs {
		out[i] = domain.KnownHost{ID: r.ID, Host: r.Host, Keys: r.Keys, Fingerprints: fingerprints(r.Keys), CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// fingerprints returns "<type> SHA256:<hash>" per parseable line.
func fingerprints(keys string) []string {
	var out []string
	rest := []byte(keys)
	for len(rest) > 0 {
		var key ssh.PublicKey
		var err error
		_, _, key, _, rest, err = ssh.ParseKnownHosts(rest)
		if err != nil {
			break
		}
		out = append(out, key.Type()+" "+ssh.FingerprintSHA256(key))
	}
	sort.Strings(out)
	return out
}

func (KnownHosts) Forget(ctx context.Context, guildID, id uint64) (bool, error) {
	res, err := facades.Orm().WithContext(ctx).Query().Where("guild_id", guildID).Where("id", id).Delete(&knownHostRecord{})
	if err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}
