package benchmarks

import (
	"fmt"
	"gogogo/modules"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkServerWithMetrics(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	metricsMiddleware := modules.MetricsMiddleware(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		metricsMiddleware.ServeHTTP(w, req)
	}
}

func BenchmarkServerWithoutMetrics(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func TestMetricsOverhead(t *testing.T) {
	withMetrics := testing.Benchmark(BenchmarkServerWithMetrics)
	withoutMetrics := testing.Benchmark(BenchmarkServerWithoutMetrics)

	overhead := float64(withMetrics.NsPerOp()-withoutMetrics.NsPerOp()) / float64(withoutMetrics.NsPerOp()) * 100

	fmt.Printf("Performance overhead of metrics: %.2f%%\n", overhead)
	fmt.Printf("Time per operation with metrics: %s\n", time.Duration(withMetrics.NsPerOp()))
	fmt.Printf("Time per operation without metrics: %s\n", time.Duration(withoutMetrics.NsPerOp()))
	fmt.Printf("Memory allocation with metrics: %d bytes\n", withMetrics.AllocsPerOp())
	fmt.Printf("Memory allocation without metrics: %d bytes\n", withoutMetrics.AllocsPerOp())
}
