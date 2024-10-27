package metrics

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu                  sync.RWMutex
	ServerMetrics       ServerMetrics
	RequestMetrics      [100]RequestMetric
	requestMetricsIndex int32
}

type ServerMetrics struct {
	TotalRequests    int64
	AverageResponse  int64 // Store as nanoseconds for atomic operations
	CacheSize        int32
	CacheHitRate     uint64 // Store as fixed-point number
	MemoryUsage      uint64
	ActiveGoroutines int32
}

type RequestMetric struct {
	URL          string
	ResponseTime time.Duration
	MemoryUsed   uint64
	StatusCode   int
}

var globalMetrics *Metrics

func init() {
	globalMetrics = &Metrics{}
}

func GetMetrics() *Metrics {
	return globalMetrics
}

func (m *Metrics) UpdateServerMetrics(metrics ServerMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ServerMetrics = metrics
}

func (m *Metrics) AddRequestMetric(metric RequestMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := m.requestMetricsIndex % 100
	m.RequestMetrics[index] = metric
	m.requestMetricsIndex++

	// Update server metrics
	atomic.AddInt64(&m.ServerMetrics.TotalRequests, 1)
	newAvg := (m.ServerMetrics.AverageResponse*int64(m.ServerMetrics.TotalRequests-1) + int64(metric.ResponseTime)) / int64(m.ServerMetrics.TotalRequests)
	atomic.StoreInt64(&m.ServerMetrics.AverageResponse, newAvg)
}

func (m *Metrics) GetServerMetrics() ServerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ServerMetrics
}

func (m *Metrics) GetRequestMetrics() [100]RequestMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.RequestMetrics
}

func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a custom ResponseWriter to capture the status code
			rw := &responseWriter{ResponseWriter: w}

			// Call the next handler
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Collect metrics
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			metric := RequestMetric{
				URL:          r.URL.Path,
				ResponseTime: duration,
				MemoryUsed:   m.Alloc,
				StatusCode:   rw.statusCode,
			}

			GetMetrics().AddRequestMetric(metric)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func UpdateCacheMetrics(size int, hitRate float64) {
	metrics := GetMetrics()
	atomic.StoreInt32(&metrics.ServerMetrics.CacheSize, int32(size))
	atomic.StoreUint64(&metrics.ServerMetrics.CacheHitRate, uint64(hitRate*1e6)) // Store as fixed-point
}

// metrics dashboard serve
func SetupMetricsAPI() {
	http.HandleFunc("/api/metrics/server", HandleServerMetrics)
	http.HandleFunc("/api/metrics/requests", HandleRequestMetrics)
}

func HandleServerMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := GetMetrics()
	serverMetrics := metrics.GetServerMetrics()

	// Convert AverageResponse from nanoseconds to a duration
	serverMetrics.AverageResponse = time.Duration(serverMetrics.AverageResponse).Nanoseconds()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverMetrics)
}

func HandleRequestMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := GetMetrics()
	requestMetrics := metrics.GetRequestMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requestMetrics)
}
