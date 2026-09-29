package config

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

// Bakery's own settings. Every one has a default that works for local
// development; see .env.example.
func init() {
	config := facades.Config()
	config.Add("bakery", map[string]any{
		// Applications without a Domain get <slug>.<domain_suffix>.
		"domain_suffix": config.Env("BAKERY_DOMAIN_SUFFIX", "localhost"),
	})
}
