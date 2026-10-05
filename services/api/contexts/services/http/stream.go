package http

import (
	"context"

	contractshttp "github.com/goravel/framework/contracts/http"

	"github.com/jevido/bakery/services/api/app/respond"
	"github.com/jevido/bakery/services/api/app/sse"
	"github.com/jevido/bakery/services/api/contexts/services/app"
)

// StreamController serves a Component's Container logs as server-sent
// events. Streams also end when shutdown is done.
type StreamController struct {
	service  *app.Service
	shutdown <-chan struct{}
}

func NewStreamController(service *app.Service, shutdown <-chan struct{}) *StreamController {
	return &StreamController{service: service, shutdown: shutdown}
}

// maxLines is Coolify's MAX_LOG_LINES: "all" lines is this many.
const maxLines = 50000

// Logs sends a Component's Container's last `lines` lines (100 by default,
// -1 for all, at most 50,000, as Coolify's Lines field), each prefixed with
// its time; with `follow=1` it goes on with new ones. It sends `end` when it
// is done, there is no Container or it stops, like a Database's logs.
func (c *StreamController) Logs(ctx contractshttp.Context) contractshttp.Response {
	sid, ok := id(ctx)
	if !ok {
		return notFound(ctx)
	}
	component := ctx.Request().Route("component")
	lines := ctx.Request().QueryInt("lines", 100)
	if lines < 0 || lines > maxLines {
		lines = maxLines
	}
	follow := ctx.Request().QueryBool("follow", false)
	reqCtx := ctx.Request().Origin().Context()
	v, err := c.service.Get(reqCtx, sid)
	if err != nil {
		return fail(ctx, err)
	}
	if _, ok := v.Component(component); !ok {
		return notFound(ctx)
	}
	stream, ok := sse.Start(ctx)
	if !ok {
		return respond.Error(ctx, contractshttp.StatusInternalServerError, "streaming is not supported")
	}
	sse.Follow(reqCtx, stream, c.shutdown, func(ctx context.Context, out func(stream, line string)) (bool, error) {
		return c.service.Logs(ctx, sid, component, follow, lines, func(stream, line string) {
			if stream == "stderr" {
				out("err", line)
			} else {
				out("out", line)
			}
		})
	})
	return nil
}
