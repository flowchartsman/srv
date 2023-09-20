package srv

import (
	"context"
	"fmt"
	"time"

	"andy.dev/srv/internal/health"
	"andy.dev/srv/log"
)

// Healthcheck is a self-reported health check.
// After startup, the Check method will be called periodically, and its result
// checked and advertised on the `/livez` route. If it returns nil, the check is
// considered to be OK, whereas an error value will mark the check as failed.
type Healthcheck interface {
	Check(context.Context, *Logger) error
}

// HealthcheckFn wraps a single [Task] function as a health check.
type HealthcheckFn TaskFn

// Check allows the HealthcheckFn to implement [Healthcheck].
func (h HealthcheckFn) Check(ctx context.Context, log *Logger) error {
	return h(ctx, log)
}

type HealthCheckOption func(hc *health.HealthCheck) error

// Interval sets the health check interval. The job will be scheduled at this
// interval. Must be greater than the check timeout. Default: 30 seconds.
func Interval(interval time.Duration) HealthCheckOption {
	return func(hc *health.HealthCheck) error {
		if interval <= 0 {
			return fmt.Errorf("interval must be greater than 0")
		}
		hc.Interval = interval
		return nil
	}
}

// Timeout sets the health check timeout. If the job takes longer than this
// to run, it will be cancelled and considered to have failed. Must be less than
// the check interval. Default: 10 seconds
func Timeout(timeout time.Duration) HealthCheckOption {
	return func(hc *health.HealthCheck) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be greater than 0")
		}
		hc.Timeout = timeout
		return nil
	}
}

// MaxFailures sets the number of acceptable failures before the health check is
// considered failed. If the check returns nil (success) before this number is
// reached, the counter will be reset, and the check will remain healthy.
// Default: 0 (no failures allowed)
func MaxFailures(maxFailures int) HealthCheckOption {
	return func(hc *health.HealthCheck) error {
		if maxFailures <= 0 {
			return fmt.Errorf("max failures must be greater than 0")
		}
		hc.MaxFailures = maxFailures
		return nil
	}
}

// AddHealthCheck adds an asynchronous self-reporting health check job whose
// status will be reported at the /livez route. Checks will be considered failed
// if they return err != nil or if they take longer than the configured timeout.
// Checks may have an optional maximum number of failures, allowing them to
// remain healthy until they fail N number of times in a row.
func (s *Srv) AddHealthCheck(ID string, healthChecker Healthcheck, options ...HealthCheckOption) error {
	// TODO: can't fatal here if the type is locked. Better to return error and
	// move Srv to unexported instance
	// s.mu.Lock()
	// defer s.mu.Unlock()
	if s.started {
		s.fatal(log.Up(1), "AddHealthCheck called after Start()")
	}
	hc := &health.HealthCheck{
		ID: ID,
		Fn: healthChecker.Check,
	}
	for _, o := range options {
		if err := o(hc); err != nil {
			return err
		}
	}
	return s.healthHandler.AddCheck(hc)
}
