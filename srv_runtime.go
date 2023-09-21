package srv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"

	"andy.dev/srv/internal/ui"
	"andy.dev/srv/log"
	"github.com/alexedwards/flow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"golang.org/x/sync/errgroup"
)

const (
	servicePort = ":8081"
)

var (
	srvHTTP             *http.Server
	srvJobs             []JobFn
	srvJobErrs          chan error
	srvShutdownHandlers []JobFn
)

func serve(serviceInfo ServiceInfo) {
	if configErr != nil {
		sFatal(noloc, "Configuration error", configErr)
	}
	mux := flow.New()

	metricHandler := promhttp.HandlerFor(srvRegistry, promhttp.HandlerOpts{
		ErrorLog: &promhttpLogger{
			srvLogger().With("logger", "metrics"),
		},
		ErrorHandling:     promhttp.ContinueOnError,
		EnableOpenMetrics: true,
	})
	mux.Handle("/metrics", metricHandler)
	if srvPushURL != "" {
		srvPusher = push.New(srvPushURL, srvInfo.Name).Gatherer(srvRegistry)
	}

	// add pprof routes
	mux.Handle("/debug/pprof", http.RedirectHandler("/debug/pprof/", http.StatusSeeOther))
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/debug/pprof/...", pprof.Index)

	// logger routes
	mux.HandleFunc("/loggers/level/:logger", srvLevelHandler.RouteLevel, "GET", "POST")
	mux.HandleFunc("/loggers/level", srvLevelHandler.RouteLevel, "GET", "POST")
	mux.HandleFunc("/loggers/list", srvLevelHandler.RouteList, "GET")
	srvLevelHandler.SetLogger(srvLogger())

	mux.Handle("/livez", srvHealth, "GET")
	srvHealth.Start(srvLogger())

	// web UI at root
	mux.Handle("/...", ui.RootHandler(ui.TmplData{
		ServiceName: fmt.Sprintf("%s v%s", serviceInfo.Name, serviceInfo.Version),
		BuildData:   getBuildData().String(),
	}))

	srvHTTP = &http.Server{
		Addr:     servicePort,
		Handler:  mux,
		ErrorLog: srvLogger().With("logger", "instrumentation_http").StdLogger(log.StdLogStatic(LevelError)),
		BaseContext: func(net.Listener) context.Context {
			return srvCtx
		},
	}

	go func() {
		if err := srvHTTP.ListenAndServe(); err != nil {
			sFatal(log.NoLocation, "failed to serve routes", "err", err)
		}
	}()

	// Set up signal monitoring to stop us if signaled. Rather than using
	// signal.NotifyContext, which would merely cancel a context, we use the
	// manual method so that we can handle it twice if necessary (i.e. if the
	// user is hands-on-keyboard testing and doesn't want to wait for the
	// shutdown to complete). Because signal delivery is NON-blocking, we need
	// enough buffer to account for both of these signals, hence the channel
	// depth of 2.
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt)

	srvJobErrs = make(chan error)
	// begin running any jobs
	if len(srvJobs) > 0 {
		eg, egctx := errgroup.WithContext(srvCtx)
		for i := range srvJobs {
			job := srvJobs[i]
			eg.Go(func() error {
				// TODO named loggers for jobs hereish
				return job(egctx, srvLogger())
			})
		}
		go func() {
			srvJobErrs <- eg.Wait()
		}()
	}

	// Wait for death with a calm stoicism.
	shutdownWatcher(signals, srvJobErrs, len(srvJobs))
}

func shutdownWatcher(signals <-chan os.Signal, jobErrs <-chan error, numJobs int) {
EVENTS:
	for {
		select {
		case err := <-jobErrs:
			if err != nil {
				sError(log.NoLocation, "job failed, shutting down", err)
				break EVENTS
			}
			numJobs--
			if numJobs == 0 {
				sInfo(log.NoLocation, "all jobs complete, shutting down")
				break EVENTS
			}
		case <-signals:
			sInfo(log.NoLocation, "received shutdown signal")
			// relinquish signal handling
			signal.Reset(os.Interrupt)
			// handle a second signal
			go func() {
				<-signals
				sInfo(noloc, "forced shutdown")
				os.Exit(130) // manual ctrl-c exitcode
			}()
			break EVENTS
		case <-srvCtx.Done():
			sInfo(log.NoLocation, "service is shutting down")
			break EVENTS
		}
	}
	srvCancel()
	shutdown()
}

func shutdown() {
	normal := true

	numHandlers := len(srvShutdownHandlers)
	if len(srvShutdownHandlers) > 0 {
		sInfo(log.NoLocation, "running shutdown handlers", "num_handlers", numHandlers)
	}

	didPanic := false
	for i, sh := range srvShutdownHandlers {
		var err error
		if didPanic {
			sWarn(noloc, "skipping handler due to previous panic")
			continue
		}
		sDebug(noloc, "running shutdown handler", "handler_number", i+1)
		// TODO: add timeout
		didPanic, err = runShutdownHandler(context.Background(), sh)
		if err != nil {
			sTermLogErr(noloc, "shutdown handler failed", err, "handler_number", i+1, "total_handlers", numHandlers)
			normal = false
		}
	}

	if srvPusher != nil {
		if err := srvPusher.Add(); err != nil {
			sTermLogErr(noloc, "failed to push to pushgateway", err)
			normal = false
		}
	}

	if normal {
		termlogWrite(noloc, "SHUTDOWN - OK")
		defer os.Exit(0)
	} else {
		termlogWrite(noloc, "SHUTDOWN - NOT OK")
		defer os.Exit(1)
	}
	termlogClose()
}

// TODO: add timeouts
func runShutdownHandler(ctx context.Context, handler JobFn) (panicked bool, err error) {
	defer func() {
		if v := recover(); v != nil {
			panicked = true
			err = fmt.Errorf("handler panicked %v", v)
		}
	}()
	return false, handler(ctx, srvLogger())
}
