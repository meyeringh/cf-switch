package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// HTTP server timeout configurations.
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 120 * time.Second
)

// Server represents the HTTP server.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
	metrics    *Metrics
}

// Metrics holds Prometheus metrics.
type Metrics struct {
	togglesTotal     *prometheus.CounterVec
	apiRequestsTotal *prometheus.CounterVec
	ruleStatusGauge  prometheus.Gauge
	cfAPIHistogram   *prometheus.HistogramVec
}

// NewMetrics creates new Prometheus metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		togglesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cf_switch_toggles_total",
				Help: "Total number of rule toggles",
			},
			[]string{"enabled"},
		),
		apiRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cf_switch_api_requests_total",
				Help: "Total number of API requests",
			},
			[]string{"method", "path", "status"},
		),
		ruleStatusGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cf_switch_rule_enabled",
				Help: "Whether the Cloudflare rule is currently enabled (1) or disabled (0)",
			},
		),
		cfAPIHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cf_switch_cloudflare_api_duration_seconds",
				Help:    "Duration of Cloudflare API calls",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
	}

	prometheus.MustRegister(m.togglesTotal)
	prometheus.MustRegister(m.apiRequestsTotal)
	prometheus.MustRegister(m.ruleStatusGauge)
	prometheus.MustRegister(m.cfAPIHistogram)

	return m
}

// NewServer creates a new HTTP server.
func NewServer(addr string, authToken string, reconciler RuleReconciler, logger *slog.Logger) *Server {
	metrics := NewMetrics()

	mux := http.NewServeMux()

	// Create handlers.
	authMiddleware := NewAuthMiddleware(authToken, logger)
	ruleHandler := NewRuleHandler(reconciler, logger)
	healthHandler := NewHealthHandler(logger)

	// Health endpoints (no auth required).
	mux.HandleFunc("/healthz", healthHandler.Health)
	mux.HandleFunc("/readyz", healthHandler.Ready)
	mux.Handle("/metrics", promhttp.Handler())

	// API endpoints (auth required).
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/v1/rule", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		ruleHandler.GetRule(w, r)
	})

	apiMux.HandleFunc("/v1/rule/enable", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		ruleHandler.ToggleRule(w, r)
	})

	apiMux.HandleFunc("/v1/rule/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		ruleHandler.UpdateHosts(w, r)
	})

	// Apply auth middleware to API routes.
	mux.Handle("/v1/", authMiddleware.Middleware(apiMux))

	// Apply metrics middleware to all routes.
	handler := metricsMiddleware(mux, metrics, logger)

	server := &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		logger:  logger,
		metrics: metrics,
	}

	return server
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// UpdateRuleMetrics updates the rule status metric.
func (s *Server) UpdateRuleMetrics(enabled bool) {
	if enabled {
		s.metrics.ruleStatusGauge.Set(1)
	} else {
		s.metrics.ruleStatusGauge.Set(0)
	}
}

// metricsMiddleware adds metrics collection to HTTP handlers.
func metricsMiddleware(next http.Handler, metrics *Metrics, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status code.
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Record metrics.
		metrics.apiRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			strconv.Itoa(wrapped.statusCode),
		).Inc()

		logger.Debug("HTTP request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"user_agent", r.Header.Get("User-Agent"),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter

	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
