package server

import (
	"log/slog"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	requestCount    atomic.Int64
	requestDuration atomic.Int64 // in microseconds
	activeRequests  atomic.Int64
	healthStatus    atomic.Int64 // 1 = healthy, 0 = unhealthy
	serverStartTime = time.Now()
)

func init() {
	healthStatus.Store(1)
}

// metricsMiddleware records request metrics.
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCount.Add(1)
		activeRequests.Add(1)
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		requestDuration.Add(int64(duration / time.Microsecond))
		activeRequests.Add(-1)
	}
}

// metricsHandler exposes Prometheus-compatible metrics at /metrics.
func metricsHandler(c *gin.Context) {
	count := requestCount.Load()
	duration := requestDuration.Load()
	active := activeRequests.Load()
	health := healthStatus.Load()
	uptime := time.Since(serverStartTime).Seconds()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, `# HELP asika_requests_total Total number of HTTP requests.
# TYPE asika_requests_total counter
asika_requests_total %d
# HELP asika_request_duration_microseconds Total HTTP request duration in microseconds.
# TYPE asika_request_duration_microseconds counter
asika_request_duration_microseconds %d
# HELP asika_active_requests Current number of active HTTP requests.
# TYPE asika_active_requests gauge
asika_active_requests %d
# HELP asika_up Health status of the server (1 = healthy, 0 = unhealthy).
# TYPE asika_up gauge
asika_up %d
# HELP asika_uptime_seconds Server uptime in seconds.
# TYPE asika_uptime_seconds counter
asika_uptime_seconds %.0f
# HELP asika_goroutines Current number of goroutines.
# TYPE asika_goroutines gauge
asika_goroutines %d
# HELP asika_memory_alloc_bytes Currently allocated memory in bytes.
# TYPE asika_memory_alloc_bytes gauge
asika_memory_alloc_bytes %d
# HELP asika_memory_sys_bytes Total memory obtained from OS in bytes.
# TYPE asika_memory_sys_bytes gauge
asika_memory_sys_bytes %d
# HELP asika_gc_runs_total Total number of GC runs.
# TYPE asika_gc_runs_total counter
asika_gc_runs_total %d
`, count, duration, active, health, uptime, runtime.NumGoroutine(),
		m.Alloc, m.Sys, m.NumGC)
}

// SetHealth sets the health status for metrics.
func SetHealth(ok bool) {
	if ok {
		healthStatus.Store(1)
	} else {
		healthStatus.Store(0)
	}
}

// GetMetrics returns current metrics values for health check use.
func GetMetrics() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]interface{}{
		"requests_total":     requestCount.Load(),
		"active_requests":    activeRequests.Load(),
		"uptime_seconds":     time.Since(serverStartTime).Seconds(),
		"goroutines":         runtime.NumGoroutine(),
		"memory_alloc_bytes": m.Alloc,
		"gc_runs":            m.NumGC,
	}
}

// LogMetrics periodically logs server metrics.
func LogMetrics(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			metrics := GetMetrics()
			slog.Info("server metrics",
				"requests_total", metrics["requests_total"],
				"active_requests", metrics["active_requests"],
				"goroutines", metrics["goroutines"],
				"memory_alloc_mb", float64(metrics["memory_alloc_bytes"].(uint64))/1024/1024,
				"uptime_s", metrics["uptime_seconds"],
			)
		}
	}()
}
