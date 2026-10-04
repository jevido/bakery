package infra

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jevido/bakery/services/api/app/facades"
	"github.com/jevido/bakery/services/api/contexts/deployments/app"
	"github.com/jevido/bakery/services/api/contexts/deployments/domain"
)

type logRecord struct {
	ID           uint64 `gorm:"primaryKey"`
	DeploymentID uint64
	Stream       string
	Line         string
	CreatedAt    time.Time
}

func (logRecord) TableName() string { return "deployment_logs" }

const (
	flushEvery = 250 * time.Millisecond
	flushAt    = 100
	// maxLine keeps one runaway line (a minified bundle echoed by a build)
	// from bloating the log.
	maxLine = 8 << 10
)

type Logs struct{}

func (Logs) Writer(deploymentID uint64) app.LogWriter {
	w := &logWriter{deploymentID: deploymentID, done: make(chan struct{}), stopped: make(chan struct{})}
	go w.loop()
	return w
}

func (Logs) After(ctx context.Context, deploymentID, afterID uint64, limit int) ([]domain.LogLine, error) {
	var recs []logRecord
	err := facades.Orm().WithContext(ctx).Query().
		Where("deployment_id", deploymentID).Where("id > ?", afterID).
		OrderBy("id").Limit(limit).Find(&recs)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LogLine, len(recs))
	for i, r := range recs {
		out[i] = domain.LogLine{ID: r.ID, Stream: r.Stream, Line: r.Line, At: r.CreatedAt}
	}
	return out, nil
}

// logWriter batches lines: they are inserted every 250ms, or at once when
// 100 are waiting, so a chatty build does not cost an INSERT per line.
type logWriter struct {
	deploymentID uint64
	mu           sync.Mutex
	pending      []logRecord
	err          error
	done         chan struct{}
	stopped      chan struct{}
	once         sync.Once
}

func (w *logWriter) Line(stream, line string) {
	if len(line) > maxLine {
		line = line[:maxLine] + " …(truncated)"
	}
	if !utf8.ValidString(line) {
		line = strings.ToValidUTF8(line, "\uFFFD") // Postgres text refuses invalid UTF-8
	}
	w.mu.Lock()
	w.pending = append(w.pending, logRecord{DeploymentID: w.deploymentID, Stream: stream, Line: line, CreatedAt: time.Now()})
	full := len(w.pending) >= flushAt
	w.mu.Unlock()
	if full {
		w.flush()
	}
}

func (w *logWriter) loop() {
	defer close(w.stopped)
	t := time.NewTicker(flushEvery)
	defer t.Stop()
	for {
		select {
		case <-w.done:
			w.flush()
			return
		case <-t.C:
			w.flush()
		}
	}
}

func (w *logWriter) flush() {
	w.mu.Lock()
	batch := w.pending
	w.pending = nil
	w.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	// Inserted in arrival order, so ids keep the lines in order. A batch
	// runs to completion even after the Deployment's context ended.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := facades.Orm().WithContext(ctx).Query().Create(&batch); err != nil {
		w.mu.Lock()
		w.err = err
		w.mu.Unlock()
	}
}

// Close flushes what is left and reports the first write error.
func (w *logWriter) Close() error {
	w.once.Do(func() { close(w.done) })
	<-w.stopped
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}
