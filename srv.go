package srv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"andy.dev/srv/buildinfo"
	"andy.dev/srv/internal/health"
	"andy.dev/srv/internal/loghandler/inithandler"
	"andy.dev/srv/log"
)

var (
	srvMu      sync.Mutex
	didDeclare bool
	didServe   bool
	configErr  error
)

var (
	srvInfo   *ServiceInfo
	srvCtx    context.Context
	srvCancel context.CancelFunc
)

func Declare(serviceInfo ServiceInfo) {
	caller := log.Up(1)
	srvMu.Lock()
	defer srvMu.Unlock()
	if didDeclare {
		sWarn(caller, "Declare(): ignoring duplicate call")
	}
	if buildinfo.Version != "" {
		if serviceInfo.Version != "" {
			sFatal(caller, "version specified in build tags, but manual version provided")
		}
		serviceInfo.Version = buildinfo.Version
	}
	if err := validateInfo(serviceInfo); err != nil {
		sFatal(caller, "Declare():", err)
	}
	srvInfo = &serviceInfo
	srvlogger.Store(srvLogger().With("service", serviceInfo))
	didDeclare = true
}

// Serve serves all endpoints and begins running all tasks. It will block until
// all tasks have completed or until the service is shut down.
func Serve() {
	caller := log.Up(1)
	srvMu.Lock()
	defer srvMu.Unlock()
	if didServe {
		// already have the package instance. Use it to log a warning
		sWarn(caller, "Serve(): ignoring duplicate call")
		return
	}

	if !didDeclare {
		sFatal(caller, "Declare() not called before Serve()")
	}

	// ensure --help or --usage will fire and identify unparsed user flags
	_, err := parseFlags()
	switch {
	case err == nil:
		if hasUserFlags {
			// user defined flags, but didn't parse them
			sFatal(caller, "ParseFlags() not called before Serve(), user flags could be invalid")
		}
	case errors.Is(err, errAlreadyParsed):
		// no problem
	case err != nil:
		sFatal(caller, "Serve()", err)
	}
	serve(*srvInfo)
	didServe = true
}

// AddHealthCheck adds an asynchronous self-reporting health check job whose
// status will be reported at the /livez route. Checks will be considered failed
// if they return err != nil or if they take longer than the configured timeout.
// Checks may have an optional maximum number of failures, allowing them to
// remain healthy until they fail N number of times in a row.
func AddHealthCheck(ID string, checkFn JobFn, options ...HealthCheckOption) {
	caller := log.Up(1)
	hc := &health.HealthCheck{
		ID: ID,
		Fn: checkFn,
	}
	for _, o := range options {
		if err := o(hc); err != nil {
			sFatal(caller, "bad health check option", err)
		}
	}
	if err := srvHealth.AddCheck(hc); err != nil {
		sFatal(caller, "failed to add health check", err)
	}
}

// AddShutdownHandler adds a job that will be run when the service is shut down.
// Shutdown handlers will be run synchronously, in the order they are defined.
// If a shutdown handler panics, the rest of the handlers will be skipped.
func AddShutdownHandler(handlers ...JobFn) {
	srvMu.Lock()
	defer srvMu.Unlock()
	srvShutdownHandlers = append(srvShutdownHandlers, handlers...)
}

// AddJob adds a function to be run at startup as an asynchronous task or as
// your main loop.
// The return value is treated as indicating success (nil) or failure (a non-nil
// error value). A return value of nil is assumed to be the successful
// completion of a short-running job. If any function returns a non-nil error
// value, the service will cancel all outstanding contexts, shut down, and will
// exit with a failure code. If all jobs return nil, the service will shut down,
// but exit with code 0, assuming success. As long as at least one function is
// outstanding and no jobs have failed, the service will continue to run.
func AddJob(jobs ...JobFn) {
	srvMu.Lock()
	defer srvMu.Unlock()
	srvJobs = append(srvJobs, jobs...)
}

// AddJobFn is the [Fn] version of [AddJob].
// Allows an edditional argument for injecting dependencies and such.
func AddJobFn[T any](fn func(context.Context, *Logger, T) error, arg T) {
	srvMu.Lock()
	defer srvMu.Unlock()
	srvJobs = append(srvJobs, func(ctx context.Context, logger *Logger) error {
		return fn(ctx, logger, arg)
	})
}

// Fn creates a JobFn from a function that takes a context and a [*Logger]
// along with an additional argument of any type.
// This argument is passed as the second argument to Fn.
//
// Example:
//
//	func myjob(ctx context.Context, log *srv.Logger input *MyType) error {
//	  // ...
//	}
//
//	srv.AddJob(srv.Fn(myjob, &MyType{/*...*/}))
func Fn[T any](fn func(context.Context, *Logger, T) error, arg T) JobFn {
	return func(ctx context.Context, logger *Logger) error {
		return fn(ctx, logger, arg)
	}
}

// Fatal logs a structured message at the error level with the root logger and
// exits the program immediately.
//
// NOTE: This bypasses any graceful shutdown handling,  so its use outside of
// main() is highly discouraged.
func Fatal(msg string, attrs ...any) {
	caller := log.Up(1)
	sFatal(caller, msg, attrs...)
}

// Fatalf logs a formatted message at the error level with the root logger and
// exits the program immediately.
//
// NOTE: This bypasses any graceful shutdown handling,  so its use outside of
// main() is highly discouraged.
func Fatalf(format string, args ...any) {
	caller := log.Up(1)
	sFatal(caller, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func initCtx() {
	srvCtx, srvCancel = context.WithCancel(context.Background())
	// set up a basic logger for before we can set the user's logging handler.
}

func init() {
	termlogErr := initTermLog()
	initCtx()
	srvlogger.Store(log.NewLogger(slog.New(inithandler.New())))
	config, err := initConfig()
	if err != nil {
		configErr = err
	}
	if config.flags != nil {
		srvFlags = config.flags
	}
	initMetrics()
	initLogging(config)
	initHealth()
	if termlogErr != nil {
		sWarn(noloc, "could not open termination log", termlogErr, "termlog_path", termlogPath)
	}
}
