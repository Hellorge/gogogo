package benchmarks

import (
	"fmt"
	"gogogo/modules"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func simpleHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

// Complex handler (simulating database operation)
func complexHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(100))) // Simulate DB operation
	fmt.Fprintf(w, "Complex operation result")
}

// Large data handler
func largeDataHandler(w http.ResponseWriter, r *http.Request) {
	data := make([]byte, 1024*1024) // 1MB of data
	rand.Read(data)
	w.Write(data)
}

func runBenchmark(b *testing.B, handler http.HandlerFunc, withMetrics bool) {
	if withMetrics {
		handler = modules.MetricsMiddleware(handler)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkSimpleHandler(b *testing.B) {
	b.Run("WithoutMetrics", func(b *testing.B) { runBenchmark(b, simpleHandler, false) })
	b.Run("WithMetrics", func(b *testing.B) { runBenchmark(b, simpleHandler, true) })
}

func BenchmarkComplexHandler(b *testing.B) {
	b.Run("WithoutMetrics", func(b *testing.B) { runBenchmark(b, complexHandler, false) })
	b.Run("WithMetrics", func(b *testing.B) { runBenchmark(b, complexHandler, true) })
}

func BenchmarkLargeDataHandler(b *testing.B) {
	b.Run("WithoutMetrics", func(b *testing.B) { runBenchmark(b, largeDataHandler, false) })
	b.Run("WithMetrics", func(b *testing.B) { runBenchmark(b, largeDataHandler, true) })
}

func BenchmarkConcurrentRequests(b *testing.B) {
	handler := http.HandlerFunc(simpleHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	b.Run("WithoutMetrics", func(b *testing.B) {
		var wg sync.WaitGroup
		for i := 0; i < b.N; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				http.Get(server.URL)
			}()
		}
		wg.Wait()
	})

	serverWithMetrics := httptest.NewServer(modules.MetricsMiddleware(handler))
	defer serverWithMetrics.Close()

	b.Run("WithMetrics", func(b *testing.B) {
		var wg sync.WaitGroup
		for i := 0; i < b.N; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				http.Get(serverWithMetrics.URL)
			}()
		}
		wg.Wait()
	})
}

func TestComprehensiveMetricsOverhead(t *testing.T) {
	benchmarks := []struct {
		name      string
		benchFunc func(*testing.B)
	}{
		{"SimpleHandler", BenchmarkSimpleHandler},
		{"ComplexHandler", BenchmarkComplexHandler},
		{"LargeDataHandler", BenchmarkLargeDataHandler},
		{"ConcurrentRequests", BenchmarkConcurrentRequests},
	}

	for _, bm := range benchmarks {
		t.Run(bm.name, func(t *testing.T) {
			resultWithout := testing.Benchmark(func(b *testing.B) { bm.benchFunc(b) })
			resultWith := testing.Benchmark(func(b *testing.B) { bm.benchFunc(b) })

			overhead := float64(resultWith.NsPerOp()-resultWithout.NsPerOp()) / float64(resultWithout.NsPerOp()) * 100

			fmt.Printf("%s - Performance overhead: %.2f%%\n", bm.name, overhead)
			fmt.Printf("  Time per op (without metrics): %s\n", time.Duration(resultWithout.NsPerOp()))
			fmt.Printf("  Time per op (with metrics): %s\n", time.Duration(resultWith.NsPerOp()))
			fmt.Printf("  Memory alloc (without metrics): %d bytes\n", resultWithout.AllocsPerOp())
			fmt.Printf("  Memory alloc (with metrics): %d bytes\n", resultWith.AllocsPerOp())
		})
	}
}
