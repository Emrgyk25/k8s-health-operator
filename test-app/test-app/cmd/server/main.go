package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	appErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_error_total",
			Help: "Total number of simulated application errors.",
		},
		[]string{"severity", "error_code"},
	)

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_app_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"path", "method", "status"},
	)

	memoryHolder [][]byte
	mu           sync.Mutex
)

func init() {
	prometheus.MustRegister(appErrorsTotal)
	prometheus.MustRegister(httpRequestsTotal)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()

	mux.HandleFunc("/health", withMetrics("/health", healthHandler(logger)))
	mux.HandleFunc("/ready", withMetrics("/ready", readyHandler(logger)))
	mux.HandleFunc("/error", withMetrics("/error", errorHandler(logger)))
	mux.HandleFunc("/panic", withMetrics("/panic", panicHandler(logger)))
	mux.HandleFunc("/cpu", withMetrics("/cpu", cpuHandler(logger)))
	mux.HandleFunc("/memory", withMetrics("/memory", memoryHandler(logger)))
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":8080"
	logger.Info("starting test application", "addr", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("server failed", "error", err.Error())
		os.Exit(1)
	}
}

func healthHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "healthy",
		})
	}
}

func readyHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ready",
		})
	}
}

func errorHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errorCode := "POD_RESTART_REQUIRED"

		appErrorsTotal.WithLabelValues("critical", errorCode).Inc()

		logger.Error(
			"simulated critical error",
			"severity", "critical",
			"error_code", errorCode,
			"action", "restart_pod",
		)

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":     "error",
			"severity":   "critical",
			"error_code": errorCode,
			"message":    "simulated critical error",
		})
	}
}

func panicHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Error("simulated panic requested")
		panic("simulated application panic")
	}
}

func cpuHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("simulating cpu load")

		go func() {
			end := time.Now().Add(30 * time.Second)
			for time.Now().Before(end) {
				_ = fmt.Sprintf("%d", time.Now().UnixNano())
			}
		}()

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "cpu load started for 30 seconds",
		})
	}
}

func memoryHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("simulating memory allocation")

		mu.Lock()
		defer mu.Unlock()

		chunk := make([]byte, 50*1024*1024)
		memoryHolder = append(memoryHolder, chunk)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		writeJSON(w, http.StatusOK, map[string]any{
			"status":             "memory allocated",
			"allocated_mb":       len(memoryHolder) * 50,
			"runtime_allocated":  m.Alloc,
			"runtime_sys_memory": m.Sys,
		})
	}
}

func withMetrics(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next(recorder, r)

		httpRequestsTotal.WithLabelValues(
			path,
			r.Method,
			fmt.Sprintf("%d", recorder.statusCode),
		).Inc()
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}