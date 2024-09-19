package metrics

import (
    "encoding/json"
    "net/http"
    "runtime"
    "sync"
    "time"
    "sync/atomic"
)

var (
    requestMetrics     [100]RequestMetric
    requestMetricsIndex int32
    serverMetrics      ServerMetrics
    serverMetricsMutex sync.RWMutex
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

func Init() {
    http.HandleFunc("/metrics/requests", handleRequestMetrics)
    http.HandleFunc("/metrics/server", handleServerMetrics)
}

func handleRequestMetrics(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(requestMetrics[:])
}

func handleServerMetrics(w http.ResponseWriter, r *http.Request) {
    serverMetricsMutex.RLock()
    metrics := serverMetrics
    serverMetricsMutex.RUnlock()
    
    // Convert average response time back to Duration
    metrics.AverageResponse = time.Duration(atomic.LoadInt64(&metrics.AverageResponse)).Nanoseconds()
    
    json.NewEncoder(w).Encode(metrics)
}

func updateMetrics(url string, responseTime time.Duration, memoryUsed uint64, statusCode int) {
    index := int(atomic.AddInt32(&requestMetricsIndex, 1) - 1) % 100
    requestMetrics[index] = RequestMetric{
        URL:          url,
        ResponseTime: responseTime,
        MemoryUsed:   memoryUsed,
        StatusCode:   statusCode,
    }

    atomic.AddInt64(&serverMetrics.TotalRequests, 1)
    
    // Update average response time
    currentAvg := atomic.LoadInt64(&serverMetrics.AverageResponse)
    newAvg := (currentAvg*int64(serverMetrics.TotalRequests-1) + responseTime.Nanoseconds()) / int64(serverMetrics.TotalRequests)
    atomic.StoreInt64(&serverMetrics.AverageResponse, newAvg)

    atomic.StoreUint64(&serverMetrics.MemoryUsage, memoryUsed)
    atomic.StoreInt32(&serverMetrics.ActiveGoroutines, int32(runtime.NumGoroutine()))
}

func Middleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        rw := &responseWriter{ResponseWriter: w}
        
        next.ServeHTTP(rw, r)
        
        duration := time.Since(start)
        
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        updateMetrics(r.URL.Path, duration, m.Alloc, rw.statusCode)
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
    atomic.StoreInt32(&serverMetrics.CacheSize, int32(size))
    atomic.StoreUint64(&serverMetrics.CacheHitRate, uint64(hitRate * 1e6)) // Store as fixed-point
}