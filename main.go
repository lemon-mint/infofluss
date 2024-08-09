package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lemon-mint/envaddr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	config, err := LoadConfig("config.jsonnet")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	s, err := NewServer(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create server")
	}

	// Create a new http.Server
	server := &http.Server{
		Addr:        envaddr.Get(":8080"),
		Handler:     s,
		IdleTimeout: 20 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		log.Info().Msg("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Error starting server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}
