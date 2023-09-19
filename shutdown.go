package srv

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"andy.dev/srv/log"
)

// AddShutdownHandler adds a job that will be run when the service is shut down.
// Shutdown handlers will be run synchronously, in the order they are defined.
// If a shutdown handler panics, the rest of the handlers will be skipped.
//
// All shutdown handlers must be added before [*srv.Start()] is called.
func (s *Srv) AddShutdownHandlers(shutdownHandlers ...JobFn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return fmt.Errorf("can't add shutdown handlers after Start() is called")
	}
	s.shutdownHandlers = append(s.shutdownHandlers, shutdownHandlers...)
	return nil
}

func (s *Srv) shutdownWatcher(signals <-chan os.Signal, jobErrs <-chan error, numJobs int) {
EVENTS:
	for {
		select {
		case err := <-jobErrs:
			if err != nil {
				s.error(log.NoLocation, "job failed, shutting down", err)
				break EVENTS
			}
			numJobs--
			if numJobs == 0 {
				s.info(log.NoLocation, "all jobs complete, shutting down")
				break EVENTS
			}
		case <-signals:
			s.info(log.NoLocation, "received shutdown signal")
			// relinquish signal handling
			signal.Reset(os.Interrupt)
			// handle a second signal
			go func() {
				<-signals
				s.info(noloc, "forced shutdown")
				os.Exit(130) // manual ctrl-c exitcode
			}()
			break EVENTS
		case <-s.ctx.Done():
			s.info(log.NoLocation, "service is shutting down")
			break EVENTS
		}
	}
	s.cancel()
	s.shutdown()
}

func (s *Srv) shutdown() {
	normal := true

	numHandlers := len(s.shutdownHandlers)
	if len(s.shutdownHandlers) > 0 {
		s.info(log.NoLocation, "running shutdown handlers", "num_handlers", numHandlers)
	}

	didPanic := false
	for i, sh := range s.shutdownHandlers {
		var err error
		if didPanic {
			s.warn(noloc, "skipping handler due to previous panic")
			continue
		}
		s.debugf(noloc, "running shutdown handler %d/%d", i+1, numHandlers)
		// TODO: add timeout
		didPanic, err = s.runShutdownHandler(context.Background(), sh)
		if err != nil {
			s.termLogErr(noloc, "shutdown handler failed", err, "handler_number", i+1, "total_handlers", numHandlers)
			normal = false
		}
	}

	if s.pusher != nil {
		if err := s.pusher.Add(); err != nil {
			s.termLogErr(noloc, "failed to push to pushgateway", err)
			normal = false
		}
	}

	if normal {
		s.termLog(log.NoLocation, "SHUTDOWN - OK")
		os.Exit(0)
	} else {
		s.termLog(log.NoLocation, "SHUTDOWN - NOT OK")
		os.Exit(1)
	}
	s.closeTermlog()
}

// TODO: add timeouts
func (s *Srv) runShutdownHandler(ctx context.Context, handler JobFn) (panicked bool, err error) {
	defer func() {
		if v := recover(); v != nil {
			panicked = true
			err = fmt.Errorf("handler panicked %v", v)
		}
	}()
	return false, handler(ctx, s.logger)
}
