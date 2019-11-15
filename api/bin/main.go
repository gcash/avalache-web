package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gcash/sherpa/api"
)

func main() {
	// Create new server
	server, err := api.New()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listening on port %d...\n", server.Port())

	// Wait for shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Stop
	err = server.Stop()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Stopped gracefully")
}
