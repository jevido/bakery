package routes

import (
	"github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
)

func Api() {
	facades.Route().Get("/api/health", func(ctx http.Context) http.Response {
		if err := facades.DB().Select(&[]struct{ One int }{}, "SELECT 1 AS one"); err != nil {
			return ctx.Response().Json(http.StatusServiceUnavailable, http.Json{"ok": false, "error": "database unreachable"})
		}
		return ctx.Response().Json(http.StatusOK, http.Json{"ok": true})
	})
}
