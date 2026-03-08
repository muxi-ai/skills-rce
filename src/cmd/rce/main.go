package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/muxi-ai/skills-rce/pkg/api"
	"github.com/muxi-ai/skills-rce/pkg/cache"
	"github.com/muxi-ai/skills-rce/pkg/config"
)

//go:embed version.txt
var embeddedVersion string

// Set via ldflags: -ldflags "-X main.Version=..."
var Version string

func getVersion() string {
	if Version != "" {
		return Version
	}
	return strings.TrimSpace(embeddedVersion)
}

func main() {
	version := getVersion()
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("service", "skills-rce").Logger()

	cfg := config.Load()

	cm, err := cache.NewManager(cfg.CacheDir)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize cache manager")
	}

	srv := api.NewServer(cfg, cm, &logger, version)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info().Msg("shutdown signal received")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("server shutdown error")
		}
	}()

	logger.Info().Int("port", cfg.Port).Str("version", version).Msg("starting skills-rce server")

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("server failed")
	}

	<-ctx.Done()
	logger.Info().Msg("server stopped")
}
