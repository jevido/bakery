// Package respond gives every context the same JSON error shape:
// {"message": "...", "errors": {"field": "..."}}. The dashboard relies on it.
package respond

import (
	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/facades"
)

// Error answers with a message and no field errors.
func Error(ctx contractshttp.Context, status int, message string) contractshttp.AbortableResponse {
	return ctx.Response().Json(status, contractshttp.Json{"message": message})
}

// Invalid answers 422 with the message on one field.
func Invalid(ctx contractshttp.Context, field, message string) contractshttp.AbortableResponse {
	return ctx.Response().Json(contractshttp.StatusUnprocessableEntity, contractshttp.Json{
		"message": message,
		"errors":  map[string]string{field: message},
	})
}

// BadBody answers a request whose body is not the JSON the route expects.
func BadBody(ctx contractshttp.Context) contractshttp.AbortableResponse {
	return Error(ctx, contractshttp.StatusBadRequest, "request body must be JSON")
}

// ServerError logs err and answers 500 without leaking it.
func ServerError(ctx contractshttp.Context, err error) contractshttp.AbortableResponse {
	facades.Log().WithContext(ctx.Context()).Error(err)
	return Error(ctx, contractshttp.StatusInternalServerError, "something went wrong")
}
