package srv

import (
	"net/http"
	"sync/atomic"
)

var defaultReadinessHandler = &readinessHandler{}

func Started() {
	if defaultReadinessHandler.setReady() {
		srvInfof(noLocation, "service has started")
	}
}

func UnavailableWhile(reason string, f func() error) error {
	srvInfo(location(1), "service has become unavailable", "reason", reason)
	defaultReadinessHandler.add()
	res := f()
	defaultReadinessHandler.sub()
	srvInfo(location(1), "service has become available again", "reason", reason)
	return res
}

type readinessHandler struct {
	isReady     atomic.Bool
	isClosed    atomic.Bool
	outstanding atomic.Int64
}

// false if already ready
// could double-log if called concurrently for the first time, but eh...
func (rh *readinessHandler) setReady() bool {
	if rh.isReady.Load() {
		return false
	}
	rh.isReady.Store(true)
	return true
}

func (rh *readinessHandler) add() {
	rh.outstanding.Add(1)
}

func (rh *readinessHandler) sub() {
	rh.outstanding.Add(-1)
}

func (rh *readinessHandler) close() {
	rh.isClosed.Store(true)
}

func (rh *readinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if rh.isClosed.Load() || rh.outstanding.Load() > 0 || !rh.isReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
