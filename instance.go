package srv

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"

	"andy.dev/srv/buildinfo"
	"andy.dev/srv/internal/config"
	"andy.dev/srv/internal/health"
	"andy.dev/srv/internal/loghandler"
	"andy.dev/srv/internal/loghandler/instrumentation"
	"andy.dev/srv/internal/loglevelhandler"
	"andy.dev/srv/internal/ui"
	"andy.dev/srv/log"
	"golang.org/x/sync/errgroup"

	"github.com/alexedwards/flow"
	"github.com/go-kit/kit/metrics"
	promkit "github.com/go-kit/kit/metrics/prometheus"
	"github.com/peterbourgon/ff/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	WebPort             = ":8080"
	InstrumentationPort = ":8081"
	termlogFile         = `/dev/termination-log`
)

var ErrAlreadyStarted = errors.New("already started")

var noloc = log.NoLocation

type instance struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc // with reasons
	// config
	srvInfo  ServiceInfo
	srvFlags *ff.CoreFlags
	// flagProviders []*fvSite
	userFlags    *ff.CoreFlags
	hasUserFlags bool
	// logging
	logHandler      *instrumentation.Handler
	logger          *log.Logger
	termination     *os.File
	loglevelHandler *loglevelhandler.Handler
	// metrics
	registry  prometheus.Registerer
	errCount  metrics.Counter
	warnCount metrics.Counter
	infoCount metrics.Counter
	pusher    *push.Pusher
	// instrumentation
	httpSrv        *http.Server
	mux            *flow.Mux
	startupHandler *health.StartupHandler
	healthHandler  *health.Handler
	// jobs
	jobErrs chan error
	// state
	started bool
	// shutdown handlers
	shutdownHandlers []JobFn
}

// New creates the srv instance and initializes logging and basic instrumentation.
func createInstance(serviceInfo ServiceInfo) (*instance, error) {
	// start building the struct early. This is so, in case of error in this
	// step, a partial struct can be returned that will at least have
	// logging that can be used to report the error and write it to the
	// termination log.
	s := &instance{
		jobErrs: make(chan error),
	}

	// open tlFile, save error for later warn if and only if the file is there,
	// but inaccessible
	tlFile, tlogErr := os.OpenFile(termlogFile, os.O_APPEND|os.O_WRONLY, 0o644)
	switch {
	case errors.Is(tlogErr, os.ErrNotExist):
		tlogErr = nil // if it doesn't exist, we don't care
	case tlogErr == nil:
		s.termination = tlFile
	}

	if buildinfo.Version != "" {
		if serviceInfo.Version != "" {
			// return s, fmt.Errorf("manual version specified, but buildinfo.Version has been set")
			s.fatal(log.Up(1), "manual version specified, but buildinfo.Version has been set")
		}
		serviceInfo.Version = buildinfo.Version
	}
	if err := validateInfo(serviceInfo); err != nil {
		// return s, fmt.Errorf("serviceInfo invalid: %v", err)
		s.fatal(log.Up(1), "serviceInfo invalid", err)
	}
	s.srvInfo = serviceInfo
	// Get the basig config flags to set up logging.
	config, srvFlags, err := config.CommonConfigAndFlags()
	if err != nil {
		return s, err
	}
	s.srvFlags = srvFlags
	s.userFlags = ff.NewFlags(s.srvInfo.Name + " flags")

	// initialize the registry for metrics reporting
	s.registry = prometheus.NewRegistry()

	// initialize a pusher if we are going to push metrics on shutdown
	if config.PushGateway != "" {
		s.pusher = push.New(config.PushGateway, "pushgateway").Gatherer(s.registry.(*prometheus.Registry))
	}

	// by creating a new registry, we avoid the old memstats-style Go runtime
	// metrics, and can register the newer runtime/metrics driven stats instead.
	s.registry.MustRegister(collectors.NewGoCollector(
		collectors.WithGoCollectorMemStatsMetricsDisabled(),
		collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll),
	))

	errCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "error_messages",
		Help: "the total number of error messages logged",
	}, []string{"logger"})
	s.registry.MustRegister(errCount)
	s.errCount = promkit.NewCounter(errCount)
	wrnCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "warning_messages",
		Help: "the total number of warning messages logged",
	}, []string{"logger"})
	s.registry.MustRegister(wrnCount)
	s.warnCount = promkit.NewCounter(wrnCount)
	infCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "info_messages",
		Help: "the total number of info messages logged",
	}, []string{"logger"})
	s.registry.MustRegister(infCount)
	s.infoCount = promkit.NewCounter(infCount)

	var rootLevel slog.Level
	switch config.LogLevel {
	case "debug":
		rootLevel = slog.LevelDebug
	case "info":
		// info is the default
	case "warn":
		rootLevel = slog.LevelWarn
	case "error":
		rootLevel = slog.LevelError
	}
	var formatter slog.Handler
	switch config.LogFormat {
	case "json":
		formatter = loghandler.NewJSON(os.Stderr)
	case "text":
		formatter = loghandler.NewText(os.Stderr)
	case "human":
	default:
		formatter = loghandler.NewAuto(os.Stderr)
	}
	formatter = formatter.WithAttrs([]slog.Attr{slog.Any("service", serviceInfo)})

	s.logHandler = instrumentation.NewHandler(formatter, instrumentation.HandlerOptions{
		MinLevel:     rootLevel,
		ShowLocation: true,
		TrimCode:     true,
		ErrorCounter: s.errCount.With("logger", "root"),
		WarnCounter:  s.warnCount.With("logger", "root"),
		InfoCounter:  s.infoCount.With("logger", "root"),
	})
	s.logger = log.NewLogger(slog.New(s.logHandler))

	// now's as good a time as any to warn we couldn't open the termination log
	if tlogErr != nil {
		s.warn(log.NoLocation, termlogFile+" exists, but couldn't be opened for writing", "err", tlogErr)
	}

	// Set up background context to thread through everything.
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// instrumentation
	s.mux = flow.New()

	// initialize the health handler to receive health checks, but don't add the
	// route yet.
	s.healthHandler = health.NewHandler(s.ctx, s.logger)

	// initialize the loglevel handler to manage log levels, but don't add the
	// route yet
	s.loglevelHandler = loglevelhandler.NewHandler(s.logHandler, s.logger)

	// start the instrumentation server with just the startup route
	s.startupHandler = health.NewStartupHandler()
	s.mux.Handle("/startupz", s.startupHandler)

	s.httpSrv = &http.Server{
		Addr:     InstrumentationPort,
		Handler:  s.mux,
		ErrorLog: s.logger.With("logger", "instrumentatioin_http").StdLogger(log.StdLogStatic(LevelError)),
		BaseContext: func(net.Listener) context.Context {
			return s.ctx
		},
	}
	// TODO: liveness check for start not being called?
	// time.AfterFunc...
	// s.healthHandler.SetFailed("no start")
	go func() {
		err := s.httpSrv.ListenAndServe()
		if err != nil {
			s.fatal(log.NoLocation, "instrumentation server failed", "err", err)
		}
	}()
	return s, nil
}

func (s *instance) Start(jobs ...JobFn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrAlreadyStarted
	}
	if !s.userFlags.IsParsed() {
		// TODO: don't want to log internal locations if this fails, so make
		// s.parseFlags() ([]string, error)
		s.ParseFlags()
		if s.hasUserFlags {
			s.fatal(log.Up(1), "Start() called before ParseFlags(), user flags would be invalid.")
		}
	}
	// Set up signal monitoring to stop us if signaled. Rather than using
	// signal.NotifyContext, which would merely cancel a context, we use the
	// manual method so that we can handle it twice if necessary (i.e. if the
	// user is hands-on-keyboard testing and doesn't want to wait for the
	// shutdown to complete). Because signal delivery is NON-blocking, we need
	// enough buffer to account for both of these signals, hence the channel
	// depth of 2.
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt)

	// start serving metrics, if not pushing them
	if s.pusher == nil {
		metricHandler := promhttp.HandlerFor(s.registry.(*prometheus.Registry), promhttp.HandlerOpts{
			ErrorLog: &promhttpLogger{
				s.logger.With("logger", "metrics"),
			},
			ErrorHandling:     promhttp.ContinueOnError,
			EnableOpenMetrics: true,
		})
		s.mux.Handle("/metrics", metricHandler)
	}

	// setup pprof
	s.mux.Handle("/debug/pprof", http.RedirectHandler("/debug/pprof/", http.StatusSeeOther))
	s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	s.mux.HandleFunc("/debug/pprof/...", pprof.Index)

	// logger routes
	s.mux.HandleFunc("/loggers/level/:logger", s.loglevelHandler.RouteLevel, "GET", "POST")
	s.mux.HandleFunc("/loggers/level", s.loglevelHandler.RouteLevel, "GET", "POST")
	s.mux.HandleFunc("/loggers/list", s.loglevelHandler.RouteList, "GET")

	// readiness and health routes
	s.mux.Handle("/readyz", health.NewReadinessHandler(), "GET")
	s.mux.Handle("/livez", s.healthHandler, "GET")

	// web UI at root
	s.mux.Handle("/...", ui.RootHandler(ui.TmplData{
		ServiceName: s.srvInfo.Name,
		BuildData:   s.getBuildData().String(),
		ShowMetrics: s.pusher == nil,
	}))

	// finally, set ourselves as "started"
	s.startupHandler.SetStarted()

	// begin running any jobs
	if len(jobs) > 0 {
		eg, egctx := errgroup.WithContext(s.ctx)
		for i := range jobs {
			job := jobs[i]
			eg.Go(func() error {
				return job(egctx, s.logger)
			})
		}
		go func() {
			s.jobErrs <- eg.Wait()
		}()
	}

	// Wait for death.
	s.shutdownWatcher(signals, s.jobErrs, len(jobs))

	return nil
}
