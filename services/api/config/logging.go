package config

import (
	"github.com/jevido/bakery/services/api/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("logging", map[string]any{
		// Default Log Channel
		//
		// This option defines the default log channel that gets used when writing
		// messages to the logs. The name specified in this option should match
		// one of the channels defined in the "channels" configuration array.
		"default": config.Env("LOG_CHANNEL", "stack"),

		// Log Channels
		//
		// Here you may configure the log channels for your application.
		// Available Drivers: "single", "daily", "custom", "stack"
		// Available Level: "debug", "info", "warning", "error", "fatal", "panic"
		// Available Formatter: "text", "json"
		"channels": map[string]any{
			"stack": map[string]any{
				"driver":   "stack",
				"channels": []string{"daily"},
			},
			"single": map[string]any{
				"driver":    "single",
				"path":      "storage/logs/goravel.log",
				"level":     config.Env("LOG_LEVEL", "debug"),
				"print":     false,
				"formatter": "text",
			},
			"daily": map[string]any{
				"driver": "daily",
				"path":   "storage/logs/goravel.log",
				"level":  config.Env("LOG_LEVEL", "debug"),
				"days":   7,
				// LOG_PRINT also writes every line to the console; the image
				// sets it so `podman logs bakery-api` shows them.
				"print":     config.Env("LOG_PRINT", false),
				"formatter": "text",
			},
			"otel": map[string]any{
				"driver":          "otel",
				"instrument_name": config.GetString("APP_NAME", "github.com/jevido/bakery/services/api/log"),
			},
		},
	})
}
