package health

import (
	"net/http"
	"sync/atomic"
)

type StartupHandler struct {
	started atomic.Bool
}

func NewStartupHandler() *StartupHandler {
	return &StartupHandler{}
}

func (s *StartupHandler) SetStarted() {
	s.started.Store(true)
}

func (s *StartupHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	if s.started.Load() {
		w.Write([]byte(http.StatusText(http.StatusOK)))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
}
