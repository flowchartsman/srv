package srv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/KimMachineGun/automemlimit"
	"github.com/mattn/go-isatty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"andy.dev/srv/logwrap/stdlogger"
)

const (
	InstrumentationAddr = ":8081"
)

type srvOptions struct {
	logFormat            LogFormat
	logLevel             LogLevel
	withMetrics          bool
	withStartup          bool
	withProfiling        bool
	withLeadership       bool
	debugMetricsInterval time.Duration
}

type Option func(*srvOptions) error

var (
	initOnce sync.Once
	eg       *errgroup.Group
	srvMu    sync.Mutex
	// Base context, cancelled when the process receives sigint
	srvCtx           context.Context
	srvShutdown      context.CancelFunc
	shutdownComplete = make(chan struct{})
	didInit          atomic.Bool
	shutdownHandlers []funcEntry
)

// Done returns a signal channel that will be closed when the service is shut
// down.
// Deprecated: use [Go] and provideed context's done channel.
func Done() <-chan struct{} {
	return srvCtx.Done()
}

// IsDone is for when you can't use Done() in a select statement. Be wary of
// calling this in tight loops, since it does not benefit from the efficiency of
// a select statement.
func IsDone() bool {
	select {
	case <-srvCtx.Done():
		return true
	default:
		return false
	}
}

// Context returns the global service context for use in requests that should be
// cancelled when the service is stopped. For occasions where you don't want the
// process to be cancelled immediately, use a context derived from
// context.Background() instead and check Done() or IsDone() for the signal to
// exit.
func Context() context.Context {
	return srvCtx
}

// Init sets the service name and initializes logging and metrics. All logs will
// have a "service" field containing this name, and all metrics will have a
// corresponding "service" label. If the name is given in the format
// "<system name>/<service name>" an additional "system" field/label will be
// added identifying this service as part of a larger unit to make refining
// searches for things like dashboards easier.
//
// This function should be called as early as possible to ensure that logging
// and metrics are initialized correctly.
func Init(serviceName string, options ...Option) {
	if didInit.Load() {
		srvFatalf(location(1), "srv.Init() called twice")
	}
	initOnce.Do(func() {
		srvInit(serviceName, options...)
	})
}

func srvInit(serviceName string, options ...Option) {
	srvMu.Lock()
	defer srvMu.Unlock()
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		srvFatalf(location(1), "service name cannot be empty")
	}

	/*
	 * DEFAULTS AND OPTIONS
	 */
	opts := &srvOptions{
		logFormat:            FormatAuto,
		logLevel:             LevelInfo,
		withMetrics:          true,
		withStartup:          false,
		withProfiling:        true,
		withLeadership:       false,
		debugMetricsInterval: 0,
	}

	for _, option := range options {
		if err := option(opts); err != nil {
			srvFatalVal(location(1), "", err)
		}
	}

	// automatically enable metrics if they are being debug printed
	if opts.debugMetricsInterval > 0 {
		opts.withMetrics = true
	}

	features := []any{
		"metrics", opts.withMetrics,
		"health_startup", opts.withStartup,
		"profiling", opts.withProfiling,
	}

	/*
	 * MEMORY LIMIT
	 */

	// Soft memory cap will have been set at init if containerized and a memory
	// limit is set, so record it here.
	memlimit := debug.SetMemoryLimit(-1)
	if memlimit == math.MaxInt64 {
		features = append(features, slog.String("memory_limit", "none"))
	} else {
		features = append(features, slog.String("memory_limit", readableUnits(memlimit)))
	}

	/*
	 * SERVICE NAME AND LABELLING
	 */
	systemName := ""
	sli := strings.LastIndex(serviceName, `/`)
	switch sli {
	case -1:
		// no system
	case 0:
		srvFatalf(location(1), "system component of serviceName is empty")
	case len(serviceName) - 1:
		srvFatalf(location(1), "service component of serviceName is empty")
	default:
		systemName, serviceName = serviceName[:sli], serviceName[sli+1:]
	}

	var staticAttrs []slog.Attr
	// var metricsLabels prometheus.Labels

	if systemName == "" {
		staticAttrs = []slog.Attr{slog.String("service", serviceName)}
		// metricsLabels = map[string]string{"service": serviceName}
	} else {
		staticAttrs = []slog.Attr{slog.String("service", serviceName), slog.String("system", systemName)}
		// metricsLabels = map[string]string{"service": serviceName, "system": systemName}
	}

	/*
	 * METRICS
	 */
	// Set global metricRegistry
	if opts.withMetrics {
		registry.MustRegister(collectors.NewGoCollector(
			// disable old memstats-syle metrics in favor of runtime/metrics directly
			collectors.WithGoCollectorMemStatsMetricsDisabled(),
			collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll),
		))

		// registry = prometheus.WrapRegistererWith(metricsLabels, prometheus.DefaultRegisterer)
		registry.MustRegister(errCount)
	}

	/*
	 * LOGGING
	 */
	var (
		formatter slog.Handler
	)

	// initialize global logging handler/formatter. This will be given to new subloggers.
	switch opts.logFormat {
	case FormatAuto:
		formatter = newAutoHandler(os.Stdout)
	case FormatJSON:
		formatter = newJSONHandler(os.Stdout)
	case FormatKV:
		formatter = newKVHandler(os.Stdout)
	case FormatHuman:
		formatter = newHumanHandler(humanHandlerOpts{
			minlevel: slog.LevelDebug,
			doSource: true,
		}, os.Stdout)
	}

	formatter = formatter.WithAttrs(staticAttrs)

	// set the global handler with the service naming labels so ALL logs done
	// through the package will get them.
	defaultH.Store(newsrvHandler(formatter, srvHandlerOptions{
		minlevel: slog.Level(opts.logLevel),
		doCode:   true,
		trimCode: true,
		errCt:    errCount,
	}))

	/*
	 * INSTRUMENTATION AND READINESS
	 */
	instrumentation := newIndexMux()

	if opts.withMetrics {
		metricHandler := promhttp.HandlerFor(registry.(*prometheus.Registry), promhttp.HandlerOpts{
			ErrorLog:          &promhttpLogger{newSubLogger("metrics handler", opts.logLevel, MeasureErrors)},
			ErrorHandling:     promhttp.ContinueOnError,
			EnableOpenMetrics: true,
		})
		instrumentation.Handle("/metrics", metricHandler, "Prometheus Metrics")
	}

	if opts.withProfiling {
		instrumentation.HandleFunc("/debug/pprof/", pprof.Index, "Main Profiling Prefix")
		instrumentation.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline, "command line info")
		instrumentation.HandleFunc("/debug/pprof/profile", pprof.Profile, "pprof-formatted Profile /<seconds>")
		instrumentation.HandleFunc("/debug/pprof/symbol", pprof.Symbol, "get program counters")
		instrumentation.HandleFunc("/debug/pprof/trace", pprof.Trace, "run trace")
	}

	instrumentation.Handle("/readyz", defaultReadinessHandler, "readiness route")

	features = append(features, slog.String("instrumentation_addr", InstrumentationAddr))
	// TODO: Routes for
	// - index with links to the other routes
	// - loglevel changing
	instrumentationServer := &http.Server{
		Addr:     InstrumentationAddr,
		ErrorLog: stdlogger.StaticLevel(slog.LevelError)(newSubLogger("instrumentation server", LevelError, MeasureErrors)),
		Handler:  instrumentation,
	}
	// begin serving instrumentation
	go func() {
		if err := instrumentationServer.ListenAndServe(); err != http.ErrServerClosed {
			srvErrorf(noLocation, "abnormal termination of instrumentation HTTP server: %v", err)
			// TODO: shutdown (with timeout?)
			os.Exit(1)
		}
	}()
	go func() {
		<-srvCtx.Done()
		if err := instrumentationServer.Close(); err != nil {
			srvErrorf(noLocation, "failed to shut down instrumentation HTTP server: %v", err)
		}
	}()

	if opts.debugMetricsInterval > 0 {
		go func() {
			t := time.NewTicker(opts.debugMetricsInterval)
			for {
				select {
				case <-srvCtx.Done():
					return
				case <-t.C:
					metricsReq, err := http.Get("http://127.0.0.1" + InstrumentationAddr + "/metrics")
					if err != nil {
						srvErrorf(noLocation, "failed to get debug metrics")
						continue
					}
					scanner := bufio.NewScanner(metricsReq.Body)
					for scanner.Scan() {
						if strings.HasPrefix(scanner.Text(), "#") || strings.HasPrefix(scanner.Text(), "go") {
							continue
						}
						fmt.Fprintln(os.Stderr, scanner.Text())
					}
					if scanner.Err() != nil {
						srvErrorVal(noLocation, "failed to scan debug metrics", err)
					}
					metricsReq.Body.Close()
				}
			}
		}()
	}

	if opts.withStartup {
		srvInfo(noLocation, "service initialized, waiting for startup", features...)
	} else {
		defaultReadinessHandler.setReady()
		srvInfo(noLocation, "service has started", features...)
	}
	// start the shutdown watcher
	go shutdownWatcher()
	didInit.Store(true)
}

func shutdownWatcher() {
	<-srvCtx.Done()
	defaultReadinessHandler.close()
	if len(shutdownHandlers) > 0 {
		srvInfof(noLocation, "running shutdown handlers")
	}
	numHandlers := len(shutdownHandlers)
	handlerPanicked := false
	for i, sd := range shutdownHandlers {
		if handlerPanicked {
			srvWarnf(sd.location, "skipping handler due to previous panic")
			continue
		}
		srvDebugf(sd.location, "running shutdown handler %d/%d", i+1, numHandlers)
		var wg sync.WaitGroup
		wg.Add(1)
		// goroutine to catch panicking handler
		go func(hIdx int, handler funcEntry) {
			defer func() {
				if v := recover(); v != nil {
					handlerPanicked = true
					msg := "shutdown handler panicked"
					if numHandlers > hIdx+1 {
						msg += fmt.Sprintf(", skipping %d remaining handlers", numHandlers-1-hIdx)
					}
					srvErrorVal(handler.location, msg, fmt.Errorf("%v", v))
				}
				wg.Done()
			}()
			if err := handler.run(); err != nil {
				srvErrorVal(handler.location, "shugdown handler failed", err)
			} else {
				srvDebugf(handler.location, "shutdown handler %d/%d completed", hIdx+1, numHandlers)
			}
		}(i, sd)
		wg.Wait()
	}
	close(shutdownComplete)
}

// Go will run the provided function in an errorgroup. If it returns an error,
// the service will initiate shutdown.
func Go(f func(context.Context) error) {
	eg.Go(func() error {
		return f(srvCtx)
	})
}

// Run is the generic version of Go to make running single-argument functions
// easier.
func Run[T any](fn func(context.Context, T) error, arg T) {
	eg.Go(func() error {
		return fn(srvCtx, arg)
	})
}

// Wait  waits for the service to complete or. To be used in
// conjunction with [Go] or if main.main() does not block and you will be
// manually shutting it down. .This will implicitly call [Started] if it has not
// been called already.
func Wait() {
	Started()
	err := eg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		srvErrorVal(noLocation, "", err)
	}
	srvShutdown()
	<-shutdownComplete
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// AddShutdownHandler adds a function to be called at program exit. Handlers
// will be called in the order they are added, and failures will be logged,
// along with the location of the failure.
func AddShutdownHandler(shutdownHandler func() error) {
	srvMu.Lock()
	defer srvMu.Unlock()
	handlerLoc := location(1)
	srvDebugf(handlerLoc, "added shutdown handler")
	shutdownHandlers = append(shutdownHandlers, funcEntry{
		f:        shutdownHandler,
		location: handlerLoc,
	})
}

// WithLogFormat sets the logging format. Default: [FormatAuto]
func WithLogFormat(logFormat LogFormat) Option {
	return func(o *srvOptions) error {
		if logFormat <= formatBegin || logFormat >= formatEnd {
			return errors.New("invalid log format")
		}
		o.logFormat = logFormat
		return nil
	}
}

// WithLogLevel sets the minimum logging level. Messages below this level will
// not be logged. Default: [LevelInfo]
func WithLogLevel(logLevel LogLevel) Option {
	return func(o *srvOptions) error {
		if logLevel <= levelBegin || logLevel >= levelEnd {
			return errors.New("invalid log level")
		}
		o.logLevel = logLevel
		return nil
	}
}

// WithMetrics enables or disables metrics reporting. Default: true
func WithMetrics(withMetrics bool) Option {
	return func(o *srvOptions) error {
		o.withMetrics = withMetrics
		return nil
	}
}

// WithStartup enables startup monitoring if your service needs to perform
// startup before becoming active. If set to true, you will need to explicitly
// call [Started] befpre your service is marked as ready to receive traffic.
// Otherwise, [Started] is a no-op. If [WithHealth] is set to false (the
// default), the service will still serve health routes, but no checks will be
// run on demand, and the route will always return success after [Started] is called.
func WithStartup(withStartup bool) Option {
	return func(o *srvOptions) error {
		o.withStartup = withStartup
		return nil
	}
}

// DebugMetricsInterval, if set, will periodically print all service metrics to
// stderr at the specified interval. A value of <= 0 disables the feature. A value
// of < 10 seconds is an error. This will implicitly enable metrics. Default: 0 (disabled)
func DebugMetricsInterval(interval time.Duration) Option {
	return func(o *srvOptions) error {
		if interval > 0 && interval < 10*time.Second {
			return errors.New("interval must be <= 0 or >= 10 seconds")
		}
		if interval < 0 {
			o.debugMetricsInterval = 0
			return nil
		}
		o.debugMetricsInterval = interval
		return nil
	}
}

func mustHaveInit(action string) {
	if !didInit.Load() {
		srvFatalf(location(1), "cannot %s until after srv.Init() has been called", action)
	}
}

type funcEntry struct {
	f        func() error
	location uintptr
}

func (f *funcEntry) run() error {
	return f.f()
}

func readableUnits(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func isTerm(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		if isatty.IsTerminal(f.Fd()) {
			return true
		}
	}
	return false
}

func init() {
	// init a default logger for erroneous calls to logging methods
	// before logging is initialized
	initHandler := newAutoHandler(os.Stderr).WithAttrs([]slog.Attr{
		slog.String("service", "UNINITIALIZED"),
	})

	// try to get a helpful module name at least
	bi, ok := debug.ReadBuildInfo()
	path := bi.Path
	if path == "command-line-arguments" {
		path = "(go run)"
	}
	if ok {
		initHandler = initHandler.WithAttrs([]slog.Attr{
			slog.String("go_module", path),
		})
	}
	defaultH.Store(newsrvHandler(initHandler, srvHandlerOptions{
		minlevel: slog.LevelInfo,
		doCode:   false,
		errCt:    nil,
	}))

	// interrupt handler
	srvCtx, srvShutdown = context.WithCancel(context.Background())
	// errgroup
	eg, srvCtx = errgroup.WithContext(srvCtx)
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	go func() {
		<-sigint
		srvInfof(noLocation, "received shutdown signal")
		srvShutdown()
		// they are really serious
		<-sigint
		srvInfof(noLocation, "forced immediate shutdown")
		os.Exit(130) // manual ctrl-c exitcode
	}()
}
