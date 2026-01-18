package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/thetnaingtn/synrk/internal/metrics"
)

func main() {
	log.Println("Starting metrics server on :9090")

	metrics.StartMetricsServer(":9090")

	log.Println("Metrics server started. Access metrics at http://localhost:9090/metrics")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down metrics server...")
}
