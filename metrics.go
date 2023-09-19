package srv

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"andy.dev/srv/log"
	"github.com/go-kit/kit/metrics"
	promkit "github.com/go-kit/kit/metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	CounterMetricSuffix = `_total`
	TimerMetricSuffix   = `_duration_seconds`
)

// Registry returns the service prometheus registry for plugins/packages that
// can use it.
func (s *Srv) Registry() prometheus.Registerer {
	return s.registry
}

// NewCounter returns a Prometheus Counter.
//
// NAMING: all counters will be automatically suffixed with _total if not already.
// NOTE: value cardinality must match label cardinality to use .With().
func (s *Srv) NewCounter(name, help string, labelNames ...string) metrics.Counter {
	if !strings.HasSuffix(name, CounterMetricSuffix) {
		name += "_total"
	}
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labelNames)
	s.registry.MustRegister(counter)
	return promkit.NewCounter(counter)
}

// NewGauge returns a Prometheus Gauge.
//
// NAMING: Gauges should not be suffixed with "_total".
// NOTE: value cardinality must match label cardinality to use .With().
func (s *Srv) NewGauge(name, help string, labelNames ...string) metrics.Gauge {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labelNames)
	s.registry.MustRegister(gauge)
	return promkit.NewGauge(gauge)
}

// NewSummary returns a Prometheus Summary.
//
// NOTE: value cardinality must match label cardinality to use .With().
func (s *Srv) NewSummary(name, help string, labelNames ...string) metrics.Histogram {
	summary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: name,
		Help: help,
	}, labelNames)
	s.registry.MustRegister(summary)
	return promkit.NewSummary(summary)
}

// NewHistogram returns a Prometheus Histogram.
//
// NOTE: value cardinality must match label cardinality to use .With().
func (s *Srv) NewHistogram(name, help string, buckets []float64, labelNames ...string) metrics.Histogram {
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	}, labelNames)
	s.registry.MustRegister(histogram)
	return promkit.NewHistogram(histogram)
}

// Timer is a wrapped histogram  tailored to measure durations for latency
// tracking.
//
// Calling [*Timer.Span] will return a function that, when called, will create
// an observation of fractional seconds elapsed since the call to Span. This is
// particularly useful in conjunction with defer to measure the duration of a
// function call.`
//
// Example:
//
//		 requestTimer := srv.NewTimer(
//			"my_route_seconds",
//			"time in seconds to run my_route",
//	     []time.Duration{150*time.Millisecond, 300*time.Millisecond, 500*time.Millisecond, time.Second},
//			"method")
//
//			userRoute := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
//				defer requestTimer.Span()()
//				// route handler code
//			})
//
// Following prometheus convention, durations are measured in seconds.
//
// NOTE: value cardinality must match label cardinality to use .With().
type Timer struct {
	hv *prometheus.HistogramVec
}

// NewTimer returns a duration and latency observation metric backed by a
// histogram. Following Prometheus convention of measuring in base units, all
// Timers are measured in fractional seconds.
//
// NAMING: Timers will automatically be suffixed with `_duration_seconds`, so
// there is no need to supply this.
func (s *Srv) NewTimer(name, help string, buckets []time.Duration, labelNames ...string) *Timer {
	floatBuckets := make([]float64, len(buckets))
	for i := range buckets {
		floatBuckets[i] = buckets[i].Seconds()
	}
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: floatBuckets,
	}, labelNames)
	s.registry.MustRegister(h)
	return &Timer{hv: h}
}

// Span returns a function that, when called, will me
func (t *Timer) Span(labelValues ...string) TimerSpan {
	start := time.Now()
	return func() {
		t.hv.WithLabelValues(labelValues...).Observe(time.Since(start).Seconds())
	}
}

// TimerSpan is the measurement function returned from [*Timer.Span], when
// called, it will perform the measurement
type TimerSpan func()

type promhttpLogger struct {
	logger *log.Logger
}

func (pl *promhttpLogger) Println(v ...any) {
	// code inspection reveals that v is always [string, error], the latter of
	// which mught which might be prometheus.MultiErr, which is a []error. Still
	// have a fallback in case they change this signature.
	if len(v) == 2 {
		msg, mok := v[0].(string)
		err, eok := v[1].(error)
		if mok && eok {
			multiErr := prometheus.MultiError{}
			if errors.As(err, &multiErr) {
				numerr := len(multiErr)
				for i, err := range multiErr {
					pl.logger.Error(msg, err, "total_errors", numerr, "error_no", i)
				}
			}
		}
	}
	// fallback
	pl.logger.Error(fmt.Sprint(v...), nil)
}
