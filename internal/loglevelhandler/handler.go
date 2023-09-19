package loglevelhandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"andy.dev/srv/internal/loghandler/instrumentation"
	"andy.dev/srv/log"
	"github.com/alexedwards/flow"
)

// w.Header().Set("Content-Type", "text/html; charset=utf-8")
type Handler struct {
	mu          sync.RWMutex
	logger      *log.Logger
	rootHandler *instrumentation.Handler
	handlers    map[string]*instrumentation.Handler
}

func NewHandler(rootHandler *instrumentation.Handler, logger *log.Logger) *Handler {
	h := &Handler{
		logger:      logger,
		rootHandler: rootHandler,
		handlers:    map[string]*instrumentation.Handler{},
	}
	return h
}

func (h *Handler) AddLogHandler(lh *instrumentation.Handler) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, found := h.handlers[lh.Name()]; found {
		return fmt.Errorf("duplicate logger: %s", lh.Name())
	}
	h.handlers[lh.Name()] = lh
	return nil
}

func (h *Handler) RouteLevel(w http.ResponseWriter, r *http.Request) {
	loggerName := flow.Param(r.Context(), "logger")
	target := h.rootHandler
	if loggerName != "" {
		var found bool
		target, found = h.handlers[loggerName]
		if !found {
			http.Error(w, fmt.Sprintf("no such logger: %s", loggerName), http.StatusNotFound)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(target)
		return
	case http.MethodPost:
		// handle below
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	// POST
	var override bool
	if r.URL.Query().Has("override") {
		o, err := strconv.ParseBool(r.URL.Query().Get("override"))
		if err != nil {
			http.Error(w, "override param must be boolean", http.StatusBadRequest)
			return
		}
		override = o
	}
	if r.URL.Query().Has("override") && target == h.rootHandler {
		http.Error(w, "override not supported for root logger", http.StatusBadRequest)
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		h.logger.Log(context.Background(), slog.LevelError, log.NoLocation, "levelhandler - read body", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	var newLevel slog.Level
	switch strings.ToLower(string(b)) {
	case "debug":
		newLevel = slog.LevelDebug
	case "info":
		newLevel = slog.LevelInfo
	case "warn":
		newLevel = slog.LevelWarn
	case "error":
		newLevel = slog.LevelError
	default:
		http.Error(w, "invalid level - valid levels: DEBUG, INFO, WARN, ERROR", http.StatusBadRequest)
		return
	}
	currentLevel, currentOverride := target.GetLevel()

	if currentLevel == newLevel && currentOverride == override {
		http.Error(w, "no change", http.StatusNotModified)
		return
	}
	target.SetLevel(newLevel, override)
	h.logger.Info("log level set", "logger", target.Name(), "level", newLevel, "override", override)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (h *Handler) RouteList(w http.ResponseWriter, _ *http.Request) {
	type listResponse struct {
		Root       *instrumentation.Handler            `json:"root_logger"`
		SubLoggers map[string]*instrumentation.Handler `json:"subloggers"`
	}
	json.NewEncoder(w).Encode(listResponse{
		Root:       h.rootHandler,
		SubLoggers: h.handlers,
	})
}
