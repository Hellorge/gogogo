// FileAccess benchmarks
func BenchmarkFileAccess(b *testing.B) {
	fa := fileaccess.New()

	b.Run("SmallFile", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := fa.Read("/tmp/testdata/small.txt") // 1KB
			if err != nil {
				b.Fatal(err)
			}
			if len(data) != 1024 {
				b.Fatal("wrong size")
			}
		}
	})

	b.Run("LargeFile", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := fa.Read("/tmp/testdata/large.txt") // 1MB
			if err != nil {
				b.Fatal(err)
			}
			if len(data) != 1024*1024 {
				b.Fatal("wrong size")
			}
		}
	})
}

// Cache benchmarks
func BenchmarkCache(b *testing.B) {
	cache := cache.NewCache(1000)
	data := []byte("test data")

	b.Run("Set/Get/Small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i%100)
			cache.Set(key, data, time.Now().Add(time.Hour))
			val, ok := cache.Get(key)
			if !ok || len(val) != len(data) {
				b.Fatal("cache error")
			}
		}
	})
}

// Router benchmarks
func BenchmarkRouter(b *testing.B) {
	router := router.New()

	b.Run("Route/Short", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			path, ok := router.Route("/api/test")
			if !ok {
				b.Fatal("route not found")
			}
			if path != "expected/path" {
				b.Fatal("wrong path")
			}
		}
	})
}

// ContentLoader benchmarks
func BenchmarkContentLoader(b *testing.B) {
	fm := setupTestFileManager()

	b.Run("LoadContent/AllFiles", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cl := loadContent(fm, "/tmp/testdata/validpath")
			if cl.err != nil {
				b.Fatal(cl.err)
			}
		}
	})
}
