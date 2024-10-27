package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gogogo/middleware/metrics"
	"gogogo/modules/cache"
	"gogogo/modules/coalescer"
	"gogogo/modules/config"
	"gogogo/modules/router"
	"gogogo/modules/server"
)

func main() {
	flag.Parse()

	cfg, err := config.LoadConfig("web/config.toml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize cache and coalescer
	var cacheInstance *cache.Cache
	var coalescerInstance *coalescer.Coalescer

	if cfg.Server.CachingEnabled {
		cacheInstance = cache.NewCache(cfg.Cache.MaxSize)
	}

	if cfg.Server.CoalescerEnabled {
		coalescerInstance = coalescer.NewCoalescer()
	}

	// Initialize router with dependencies
	router, err := router.NewRouter(&cfg, cacheInstance, coalescerInstance)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	serverConfig := &server.Config{
		Host:              cfg.Server.Host,
		Port:              cfg.Server.Port,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		KeepAliveDuration: 3 * time.Minute,
		EnableHTTP2:       cfg.Server.EnableHTTP2,
		GracefulTimeout:   30 * time.Second,
		MaxConnsPerIP:     100,
		TCPKeepAlive:      30 * time.Second,
	}

	// Create and set up server
	srv := server.NewServer(serverConfig)
	srv.SetHandler(router)

	// Add middlewares
	if cfg.Server.MetricsEnabled {
		srv.Use(metrics.MetricsMiddleware())
	}

	// Set up graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")
		ctx := srv.Shutdown()
		log.Println("Server stopped")
		close(done)
	}()

	log.Printf("Server starting on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	<-done
}
