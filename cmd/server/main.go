// cmd/server/main.go
// SkyTrace MLAT server — replay mode.
// Replays log.txt through the full pipeline and serves WebSocket + REST API.
// Usage: go run cmd/server/main.go --log log.txt --addr :8080
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quickstart/pkg/pipeline"
)

func main() {
	logPath := flag.String("log", "log.txt", "Path to log file")
	overrides := flag.String("overrides", "location-override.json", "Sensor location overrides")
	addr := flag.String("addr", ":8080", "HTTP server address")
	realtime := flag.Bool("realtime", false, "Replay at 10x speed (otherwise instant)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	p, err := pipeline.New(*overrides)
	if err != nil {
		log.Fatalf("Failed to create pipeline: %v", err)
	}

	// Start HTTP/WebSocket server in background
	go func() {
		if err := p.StartServer(*addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)
	log.Printf("Server ready at http://localhost%s", *addr)
	log.Printf("WebSocket at ws://localhost%s/ws", *addr)
	log.Printf("REST API at http://localhost%s/api/tracks", *addr)

	// Start replay
	go func() {
		log.Printf("Starting replay of %s...", *logPath)
		if err := p.RunReplay(*logPath, *realtime); err != nil {
			log.Printf("Replay error: %v", err)
		}
		log.Println("Replay complete. Server still running — connect to see results.")
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down.")
}
