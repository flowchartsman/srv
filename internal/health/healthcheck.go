package health

import (
	"context"
	"time"

	"andy.dev/srv/log"
)

type HealthCheck struct {
	ID          string
	Fn          func(context.Context, *log.Logger) error
	Interval    time.Duration
	Timeout     time.Duration
	MaxFailures int
}
