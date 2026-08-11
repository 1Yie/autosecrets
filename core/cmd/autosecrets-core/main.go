// Command autosecrets-core is the AutoSecrets control plane service.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"autosecrets.dev/core/internal/config"
	"autosecrets.dev/core/internal/server"
)

var version = "dev"

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	if cfg.Version == "" {
		cfg.Version = version
	}

	handler := server.New(cfg, cfg.Version)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("autosecrets-core listening on %s (management %s, agent %s)", cfg.ListenAddr, cfg.ManagementBase, cfg.AgentBase)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
