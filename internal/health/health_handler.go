package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"andy.dev/srv/log"
)

const (
	defaultInterval = 30 * time.Second
	defaultTimeout  = 10 * time.Second
	healthTTL       = 1 * time.Second
)

var validCheck = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

var (
	errTimeout = errors.New("timed out")
	errClosed  = errors.New("handler was closed")
)

type checkStatus struct {
	Timestamp   time.Time `json:"timestamp"`
	Err         error     `json:"err"`
	Failures    int       `json:"failures"`
	MaxFailures int       `json:"max_failures_allowed"`
}

type CheckFn func(context.Context, *log.Logger) error

type Handler struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	mu      sync.RWMutex
	logger  *log.Logger
	status  map[string]*checkStatus
	started bool
}

func NewHandler(ctx context.Context, logger *log.Logger) *Handler {
	hctx, ccf := context.WithCancelCause(ctx)
	return &Handler{
		ctx:    hctx,
		cancel: ccf,
		logger: logger,
		status: map[string]*checkStatus{},
	}
}

func (h *Handler) AddCheck(c *HealthCheck) error {
	select {
	case <-h.ctx.Done():
		return errClosed
	default:
	}
	if !validCheck.MatchString(c.ID) {
		return fmt.Errorf("invalid health check name")
	}
	if c.Interval <= c.Timeout {
		return fmt.Errorf("interval must be greater than timeout")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, found := h.status[c.ID]; found {
		return fmt.Errorf("duplicate health check name")
	}
	interval := defaultInterval
	if c.Interval > 0 {
		interval = c.Interval
	}
	timeout := defaultTimeout
	if c.Timeout > 0 {
		timeout = c.Timeout
	}
	h.status[c.ID] = &checkStatus{
		Timestamp:   time.Now(),
		Err:         nil,
		Failures:    0,
		MaxFailures: c.MaxFailures,
	}
	go h.dispatcher(c.ID, c.Fn, interval, timeout)
	return nil
}

func (h *Handler) SetFailed(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status["srv"] = &checkStatus{
		Timestamp:   time.Now(),
		Err:         fmt.Errorf(msg),
		Failures:    1,
		MaxFailures: 1,
	}
}

func (h *Handler) dispatcher(checkID string, fn CheckFn, interval, timeout time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if failed := h.runCheck(checkID, fn, timeout); failed {
				return
			}
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *Handler) runCheck(checkID string, fn CheckFn, timeout time.Duration) (failed bool) {
	ctx, cf := context.WithTimeoutCause(h.ctx, timeout, errTimeout)
	defer cf()
	res := make(chan error, 1)
	go func() {
		res <- fn(ctx, h.logger)
	}()

	var result error

	select {
	case err := <-res:
		if errors.Is(err, context.DeadlineExceeded) {
			result = errTimeout
		} else {
			result = err
		}
	case <-ctx.Done():
		switch context.Cause(ctx) {
		case errTimeout:
			result = errTimeout
		case errClosed:
			return true
		default:
			result = context.Cause(ctx)
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	status := h.status[checkID]
	status.Timestamp = time.Now()
	status.Err = result

	if result == nil {
		if status.Failures > 0 {
			h.logger.Debug("health check has recovered", "healthcheck_id", checkID)
		}
		status.Failures = 0
		return false
	}

	status.Failures++
	if status.Failures > status.MaxFailures {
		h.logger.Error("health check has failed, service is unhealthy", "healthcheck_id", checkID)
		return true
	}
	h.logger.Warn("health check has failed, will retry", "healthcheck_id", checkID, "num_failures", status.Failures, "max_failures", status.MaxFailures)
	return false
}

func (h *Handler) Close() {
	h.cancel(errClosed)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type allChecks struct {
		Checks map[string]*checkStatus `json:"checks"`
		Status string                  `json:"status"`
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	code := http.StatusOK
	statusMsg := "OK"
	verbose := r.URL.Query().Has("verbose")
	for _, c := range h.status {
		if c.Failures >= c.MaxFailures {
			code = http.StatusInternalServerError
			statusMsg = "NOT_OK"
			break
		}
	}
	w.WriteHeader(code)
	if !verbose {
		w.Write([]byte(statusMsg))
		return
	}
	json.NewEncoder(w).Encode(allChecks{h.status, statusMsg})
}
