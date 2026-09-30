package http

import (
	"context"
	"strconv"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/app/sse"
	"github.com/jevido/bakery/services/api/contexts/databases/app"
)

// StreamController serves a Database's Container logs as server-sent
// events. Streams also end when shutdown is done.
type StreamController struct {
	service  *app.Service
	shutdown <-chan struct{}
}

func NewStreamController(service *app.Service, shutdown <-chan struct{}) *StreamController {
	return &StreamController{service: service, shutdown: shutdown}
}

// Logs follows the Database's Container: the last `tail` lines (200 by
// default), then new ones. It sends `end` when there is no Container or it
// stops, like the Application's Container logs.
func (c *StreamController) Logs(ctx contractshttp.Context) contractshttp.Response {
	dbID, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	tail, err := strconv.Atoi(ctx.Request().Query("tail", "200"))
	if err != nil || tail < 0 || tail > 5000 {
		tail = 200
	}
	reqCtx := ctx.Request().Origin().Context()
	if _, err := c.service.Get(reqCtx, dbID); err != nil {
		return fail(ctx, err)
	}
	stream, ok := sse.Start(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}
	sse.Follow(reqCtx, stream, c.shutdown, func(ctx context.Context, out func(stream, line string)) (bool, error) {
		return c.service.Logs(ctx, dbID, tail, func(stream, line string) {
			if stream == "stderr" {
				out("err", line)
			} else {
				out("out", line)
			}
		})
	})
	return nil
}
