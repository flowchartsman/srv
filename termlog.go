package srv

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"andy.dev/srv/log"
)

const (
	// termlogPath = `/dev/termination-log`
	termlogPath = `./termlog`
)

var (
	srvTermlogfile *os.File
	srvTermlog     *log.Logger
	muTermlog      sync.Mutex
)

func initTermLog() error {
	muTermlog.Lock()
	defer muTermlog.Unlock()
	tl, err := os.OpenFile(termlogPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	srvTermlog = log.NewLogger(slog.New(slog.NewTextHandler(tl, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case "time", "level":
				return slog.Attr{}
			case "source":
				sv := a.Value.Any().(*slog.Source)
				if sv.File == "" {
					return slog.Attr{}
				}
			}
			return a
		},
	})))
	return nil
}

func termlogClose() {
	muTermlog.Lock()
	defer muTermlog.Unlock()
	if srvTermlogfile != nil {
		srvTermlogfile.Close()
	}
	srvTermlogfile = nil
	srvTermlog = nil
}

func termlogWrite(loc log.CodeLocation, msg string, attrs ...any) {
	muTermlog.Lock()
	defer muTermlog.Unlock()
	if srvTermlog == nil {
		return
	}
	srvTermlog.Log(context.Background(), slog.LevelError, loc, msg, attrs...)
}
