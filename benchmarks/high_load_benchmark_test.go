package benchmarks

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    "webSPA/metrics"
    "sync"
    "sync/atomic"
    "math/rand"
    "runtime"
)

type BenchmarkResult struct {
    RequestsPerSecond float64
    AvgLatency        float64
    MemoryAllocated   uint64
}

func mixedHandler(w http.ResponseWriter, r *http.Request) {
    switch rand.Intn(3) {
    case 0:
        simpleHandler(w, r)
    case 1:
        complexHandler(w, r)
    case 2:
        largeDataHandler(w, r)
    }
}

func runLoadTest(handler http.Handler, duration time.Duration, concurrency int) BenchmarkResult {
    var (
        requestCount int64
        totalLatency int64
        wg           sync.WaitGroup
    )

    start := time.Now()
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for time.Since(start) < duration {
                req := httptest.NewRequest("GET", "/", nil)
                w := httptest.NewRecorder()
                requestStart := time.Now()
                handler.ServeHTTP(w, req)
                latency := time.Since(requestStart)
                
                atomic.AddInt64(&requestCount, 1)
                atomic.AddInt64(&totalLatency, int64(latency))
                
                time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
            }
        }()
    }
    wg.Wait()

    elapsed := time.Since(start)
    requestsPerSecond := float64(requestCount) / elapsed.Seconds()
    avgLatency := float64(totalLatency) / float64(requestCount) / float64(time.Millisecond)

    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    return BenchmarkResult{
        RequestsPerSecond: requestsPerSecond,
        AvgLatency:        avgLatency,
        MemoryAllocated:   m.TotalAlloc,
    }
}

func BenchmarkHighTraffic(b *testing.B) {
    handler := http.HandlerFunc(mixedHandler)
    handlerWithMetrics := metrics.Middleware(handler)

    duration := 10 * time.Second
    concurrency := 100

    b.Run("WithoutMetrics", func(b *testing.B) {
        result := runLoadTest(handler, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })

    b.Run("WithMetrics", func(b *testing.B) {
        result := runLoadTest(handlerWithMetrics, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })
}

func BenchmarkSustainedHighLoad(b *testing.B) {
    handler := http.HandlerFunc(mixedHandler)
    handlerWithMetrics := metrics.Middleware(handler)

    duration := 1 * time.Minute
    concurrency := 100

    b.Run("WithoutMetrics", func(b *testing.B) {
        result := runLoadTest(handler, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })

    b.Run("WithMetrics", func(b *testing.B) {
        result := runLoadTest(handlerWithMetrics, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })
}

func BenchmarkBurstTraffic(b *testing.B) {
    handler := http.HandlerFunc(mixedHandler)
    handlerWithMetrics := metrics.Middleware(handler)

    duration := 2 * time.Second
    concurrency := 500

    b.Run("WithoutMetrics", func(b *testing.B) {
        result := runLoadTest(handler, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })

    b.Run("WithMetrics", func(b *testing.B) {
        result := runLoadTest(handlerWithMetrics, duration, concurrency)
        b.ReportMetric(result.RequestsPerSecond, "requests/sec")
        b.ReportMetric(result.AvgLatency, "avg_latency_ms")
        b.ReportMetric(float64(result.MemoryAllocated), "memory_allocated_bytes")
    })
}

func TestHighLoadBenchmarks(t *testing.T) {
    benchmarks := []struct {
        name string
        benchFunc func(*testing.B)
    }{
        {"HighTraffic", BenchmarkHighTraffic},
        {"SustainedHighLoad", BenchmarkSustainedHighLoad},
        {"BurstTraffic", BenchmarkBurstTraffic},
    }

    for _, bm := range benchmarks {
        t.Run(bm.name, func(t *testing.T) {
            result := testing.Benchmark(bm.benchFunc)
            fmt.Printf("%s:\n", bm.name)
            fmt.Printf("  Requests/sec: %.2f\n", result.Extra["requests/sec"])
            fmt.Printf("  Avg Latency: %.2f ms\n", result.Extra["avg_latency_ms"])
            fmt.Printf("  Memory Allocated: %.2f MB\n", result.Extra["memory_allocated_bytes"]/(1024*1024))
        })
    }
}