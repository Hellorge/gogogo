package modules

import (
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	requestMetrics      [100]RequestMetric
	requestMetricsIndex int32
	serverMetrics       ServerMetrics
	serverMetricsMutex  sync.RWMutex
	dashboardTemplate   *template.Template
)

type RequestMetric struct {
	URL          string
	ResponseTime time.Duration
	MemoryUsed   uint64
	StatusCode   int
}

type ServerMetrics struct {
	TotalRequests    int64
	AverageResponse  int64 // Store as nanoseconds for atomic operations
	CacheSize        int32
	CacheHitRate     uint64 // Store as fixed-point number
	MemoryUsage      uint64
	ActiveGoroutines int32
}

func NewMetrics() error {
	var err error
	dashboardTemplate, err = template.ParseFiles("metrics/dashboard.html")
	if err != nil {
		return err
	}

	http.HandleFunc("/metrics/requests", AuthMiddleware(HandleRequestMetrics))
	http.HandleFunc("/metrics/server", AuthMiddleware(HandleServerMetrics))
	http.HandleFunc("/metrics/dashboard", AuthMiddleware(serveDashboard))
	return nil
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := dashboardTemplate.Execute(w, nil); err != nil {
		http.Error(w, "Error rendering dashboard", http.StatusInternalServerError)
	}
}

func HandleRequestMetrics(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(requestMetrics[:])
}

func HandleServerMetrics(w http.ResponseWriter, r *http.Request) {
	serverMetricsMutex.RLock()
	metrics := serverMetrics
	serverMetricsMutex.RUnlock()

	metrics.AverageResponse = time.Duration(atomic.LoadInt64(&metrics.AverageResponse)).Nanoseconds()

	json.NewEncoder(w).Encode(metrics)
}

func updateMetrics(url string, responseTime time.Duration, memoryUsed uint64, statusCode int) {
	index := int(atomic.AddInt32(&requestMetricsIndex, 1)-1) % 100
	requestMetrics[index] = RequestMetric{
		URL:          url,
		ResponseTime: responseTime,
		MemoryUsed:   memoryUsed,
		StatusCode:   statusCode,
	}

	atomic.AddInt64(&serverMetrics.TotalRequests, 1)

	currentAvg := atomic.LoadInt64(&serverMetrics.AverageResponse)
	newAvg := (currentAvg*int64(serverMetrics.TotalRequests-1) + responseTime.Nanoseconds()) / int64(serverMetrics.TotalRequests)
	atomic.StoreInt64(&serverMetrics.AverageResponse, newAvg)

	atomic.StoreUint64(&serverMetrics.MemoryUsage, memoryUsed)
	atomic.StoreInt32(&serverMetrics.ActiveGoroutines, int32(runtime.NumGoroutine()))
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		updateMetrics(r.URL.Path, duration, m.Alloc, rw.statusCode)
	})
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
	atomic.StoreInt32(&serverMetrics.CacheSize, int32(size))
	atomic.StoreUint64(&serverMetrics.CacheHitRate, uint64(hitRate*1e6)) // Store as fixed-point
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		expectedUsername := os.Getenv("METRICS_USERNAME")
		expectedPassword := os.Getenv("METRICS_PASSWORD")

		if subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
