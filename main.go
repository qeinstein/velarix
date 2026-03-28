package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"velarix/api"
	"velarix/core"
	"velarix/store"

	"log/slog"
)

func main() {
	// Load environment variables from .env file if it exists
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	tp, err := api.InitTracer()
	if err != nil {
		slog.Error("Failed to initialize tracer", "error", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				slog.Error("Failed to shutdown tracer", "error", err)
			}
		}()
	}

	slog.Info("Velarix | The Epistemic State Layer for AI Agents")

	dbPath := "velarix.data"

	encryptionKey := []byte(os.Getenv("VELARIX_ENCRYPTION_KEY"))
	env := os.Getenv("VELARIX_ENV")
	jwtSecret := os.Getenv("VELARIX_JWT_SECRET")
	if len(encryptionKey) == 0 {
		if env != "dev" {
			slog.Error(" [SECURITY] CRITICAL: Encryption at rest is REQUIRED in production. Set VELARIX_ENCRYPTION_KEY (16, 24, or 32 bytes) or set VELARIX_ENV=dev to override for local development.")
			os.Exit(1)
		}
		slog.Warn(" [SECURITY] WARNING: Encryption at rest is disabled (DEV MODE).")
	} else {
		slog.Info(" [SECURITY] Encryption at rest enabled", "bits", len(encryptionKey)*8)
	}

	if jwtSecret == "" && env != "dev" {
		slog.Error(" [SECURITY] CRITICAL: JWT signing secret is REQUIRED in production. Set VELARIX_JWT_SECRET or set VELARIX_ENV=dev to override for local development.")
		os.Exit(1)
	}

	badgerStore, err := store.OpenBadger(dbPath, encryptionKey)
	if err != nil {
		slog.Error("Failed to open BadgerDB", "error", err)
		os.Exit(1)
	}
	defer badgerStore.Close()

	server := &api.Server{
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		LastAccess: make(map[string]time.Time),
		SliceCache: make(map[string]*api.SliceCacheEntry),
		Store:      badgerStore,
		StartTime:  time.Now(),
	}

	configsRaw := make(map[string][]byte)
	if err := badgerStore.ReplayAll(server.Engines, configsRaw); err != nil {
		slog.Error("Failed to replay journal on startup", "error", err)
		os.Exit(1)
	}
	slog.Info("Global journal replay complete", "sessions_loaded", len(server.Engines))

	server.StartEvictionTicker()
	server.StartBackupTicker()
	server.Store.StartGC()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}

	slog.Info("Server ready", "url", "http://localhost"+port)

	if err := http.ListenAndServe(port, server.Routes()); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
