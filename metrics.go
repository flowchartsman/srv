package srv

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-kit/kit/metrics"
	promkit "github.com/go-kit/kit/metrics/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	registry prometheus.Registerer = prometheus.NewRegistry()
	errCount                       = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "error_messages_total",
		Help: "the total number of error messages logged",
	})
)

// Registry returns the service prometheus registry for plugins/packages that
// can use it.
func Registry() prometheus.Registerer {
	return registry
}

// NewGauge returns a Prometheus Counter.
//
// NOTE: label cardinality must match to use .With().
func NewCounter(name, help string, labelNames ...string) metrics.Counter {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        name,
		Help:        help,
		ConstLabels: map[string]string{},
	}, labelNames)
	registry.MustRegister(counter)
	return promkit.NewCounter(counter)
}

// NewGauge returns a Prometheus Gauge.
//
// NOTE: label cardinality must match to use .With().
func NewGauge(name, help string, labelNames ...string) metrics.Gauge {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labelNames)
	registry.MustRegister(gauge)
	return promkit.NewGauge(gauge)
}

// NewSummary returns a Prometheus Summary.
//
//   - Summary is essentially an auto-bucketing Histogram.
//
// NOTE: label cardinality must match to use .With().
func NewSummary(name, help string, labelNames ...string) metrics.Histogram {
	summary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: name,
		Help: help,
	}, labelNames)
	registry.MustRegister(summary)
	return promkit.NewSummary(summary)
}

// NewHistogram returns a Prometheus Histogram.
//
// NOTE: label cardinality must match to use .With().
func NewHistogram(name, help string, buckets []float64, labelNames ...string) metrics.Histogram {
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	}, labelNames)
	registry.MustRegister(histogram)
	return promkit.NewHistogram(histogram)
}

// LatencyChart is a histogram representing the milliseconds taken for network
// interactions with a service or dependency, or for the work done in a single
// span.
type LatencyChart interface {
	ObserveSpan(from time.Time)
	With(labelValues ...string) LatencyChart
}

// NewLatencyChart returns a new [LatencyChart].
//   - Final metric name will be in the form: `latency_<name>_ms`
//   - If metrics are disabled, returns a nop LatencyChart.
//
// NOTE: label cardinality must match to use .With().
func NewLatencyChart(name, help string, labelNames ...string) LatencyChart {
	summary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "latency_" + name + "_ms",
		Help: help,
	}, labelNames)
	registry.MustRegister(summary)
	return &latencyChartT{promkit.NewSummary(summary)}
}

type latencyChartT struct {
	h metrics.Histogram
}

func (lc *latencyChartT) ObserveSpan(from time.Time) {
	lc.h.Observe(float64(time.Since(from).Milliseconds()))
}

func (lc *latencyChartT) With(labelValues ...string) LatencyChart {
	return &latencyChartT{
		lc.h.With(labelValues...),
	}
}

type promhttpLogger struct {
	logger *slog.Logger
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
				// log them all at the exact same timestamp
				now := time.Now()
				numerr := len(multiErr)
				logRecord := slog.NewRecord(now, slog.LevelError, msg, location(1))
				logRecord.AddAttrs(slog.String(slog.MessageKey, msg), slog.Int("total_errors", numerr))
				for i, e := range multiErr {
					r := logRecord.Clone()
					r.AddAttrs(slog.String("err", e.Error()), slog.Int("error_number", i+1))
					pl.logger.Handler().Handle(srvCtx, r)
				}
				return
			}
		}
	}
	// fallback
	pl.logger.Error(fmt.Sprint(v...), nil)
}
