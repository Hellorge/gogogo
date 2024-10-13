package main

import (
	"flag"
	"fmt"
	"gogogo/modules"
	"log"
	"net/http"
	"os"
)

var (
	projectRoot = ".."
	config      = *modules.Cfg
)

func main() {
	if err := os.Chdir(projectRoot); err != nil {
		fmt.Printf("Error changing to project root directory: %v\n", err)
		os.Exit(1)
	}

	flag.Parse()

	router, err := modules.NewRouter(&config)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	var handler http.Handler = router

	if config.MetricsEnabled {
		modules.SetupMetricsAPI()
		handler = modules.MetricsMiddleware(handler)
	}

	addr := fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort)

	fmt.Printf("Server starting on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
