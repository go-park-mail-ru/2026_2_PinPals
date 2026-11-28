package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Общее количество HTTP-запросов.",
	}, []string{"method", "route", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Длительность обработки HTTP-запроса в секундах.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "route"})

	requestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Количество запросов, обрабатываемых прямо сейчас.",
	})
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		requestsInFlight.Inc()
		defer requestsInFlight.Dec()

		next.ServeHTTP(rec, r)

		route := routeLabel(r.Pattern)
		requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

func routeLabel(pattern string) string {
	if pattern == "" {
		return "unmatched"
	}
	if _, path, ok := strings.Cut(pattern, " "); ok {
		return path
	}
	return pattern
}

func RegisterPool(pool *pgxpool.Pool) {
	gauge := func(name, help string, value func() float64) {
		promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help}, value)
	}
	gauge("db_pool_acquired_connections", "Соединения пула, занятые запросами.", func() float64 { return float64(pool.Stat().AcquiredConns()) })
	gauge("db_pool_idle_connections", "Свободные соединения пула.", func() float64 { return float64(pool.Stat().IdleConns()) })
	gauge("db_pool_total_connections", "Всего соединений в пуле.", func() float64 { return float64(pool.Stat().TotalConns()) })
	gauge("db_pool_max_connections", "Максимальный размер пула.", func() float64 { return float64(pool.Stat().MaxConns()) })
}

func NewServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
