package observability

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"go-mall/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

type Metrics struct {
	registry                *prometheus.Registry
	httpRequests            *prometheus.CounterVec
	httpDuration            *prometheus.HistogramVec
	httpInFlight            prometheus.Gauge
	paymentCallbackFailures prometheus.Counter
	collectionFailures      *prometheus.CounterVec
}

func NewMetrics(db *gorm.DB) *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "go_mall",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests grouped by method, route, and status.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "go_mall",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency grouped by method and route.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.8, 1.5, 3, 5},
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "go_mall",
			Subsystem: "http",
			Name:      "in_flight_requests",
			Help:      "Current number of HTTP requests being handled.",
		}),
		paymentCallbackFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "go_mall",
			Subsystem: "payment",
			Name:      "callback_failures_total",
			Help:      "Total payment callback failures.",
		}),
		collectionFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "go_mall",
			Subsystem: "observability",
			Name:      "collection_failures_total",
			Help:      "Total failures while collecting database-backed metrics.",
		}, []string{"metric"}),
	}

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		metrics.httpRequests,
		metrics.httpDuration,
		metrics.httpInFlight,
		metrics.paymentCallbackFailures,
		metrics.collectionFailures,
	)
	if db != nil {
		registry.MustRegister(
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: "go_mall",
				Subsystem: "order",
				Name:      "pending_payment_total",
				Help:      "Current number of orders waiting for payment.",
			}, metrics.countOrders(db, model.OrderStatusPendingPayment)),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: "go_mall",
				Subsystem: "refund",
				Name:      "unknown_total",
				Help:      "Current number of refunds with an unknown result.",
			}, metrics.countRefunds(db, model.RefundStatusUnknown)),
		)
	}
	return metrics
}

func (m *Metrics) countOrders(db *gorm.DB, status int) func() float64 {
	return func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var count int64
		if err := db.WithContext(ctx).Model(&model.Order{}).Where("status = ?", status).Count(&count).Error; err != nil {
			m.collectionFailures.WithLabelValues("pending_payment_orders").Inc()
			return math.NaN()
		}
		return float64(count)
	}
}

func (m *Metrics) countRefunds(db *gorm.DB, status int) func() float64 {
	return func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var count int64
		if err := db.WithContext(ctx).Model(&model.Refund{}).Where("status = ?", status).Count(&count).Error; err != nil {
			m.collectionFailures.WithLabelValues("unknown_refunds").Inc()
			return math.NaN()
		}
		return float64(count)
	}
}

func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		m.httpInFlight.Inc()
		defer m.httpInFlight.Dec()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		m.httpRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.httpDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(startedAt).Seconds())
	}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) RecordPaymentCallbackFailure() {
	m.paymentCallbackFailures.Inc()
}
