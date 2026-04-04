package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"velarix/api"
	"velarix/core"
	"velarix/store"
	storepostgres "velarix/store/postgres"
	storeredis "velarix/store/redis"

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

	slog.Info("Velarix | Decision Integrity For AI-Assisted Internal Approvals")

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

	backend := strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_STORE_BACKEND")))
	if backend == "" {
		if strings.TrimSpace(os.Getenv("VELARIX_POSTGRES_DSN")) != "" {
			backend = "postgres"
		} else {
			backend = "badger"
		}
	}

	var runtimeStore store.RuntimeStore
	switch backend {
	case "postgres":
		pgStore, err := storepostgres.Open(context.Background(), os.Getenv("VELARIX_POSTGRES_DSN"))
		if err != nil {
			slog.Error("Failed to open Postgres store", "error", err)
			os.Exit(1)
		}

		redisAddr := strings.TrimSpace(os.Getenv("VELARIX_REDIS_URL"))
		if redisAddr == "" {
			redisAddr = strings.TrimSpace(os.Getenv("VELARIX_REDIS_ADDR"))
		}
		if redisAddr != "" {
			redisStore, err := storeredis.Open(redisAddr)
			if err != nil {
				slog.Error("Failed to open Redis store", "error", err)
				os.Exit(1)
			}
			runtimeStore = store.NewCompositeRuntimeStore(pgStore, redisStore, redisStore, redisStore, pgStore)
			slog.Info("Using shared Postgres + Redis backend", "store_backend", backend)
		} else {
			runtimeStore = store.NewCompositeRuntimeStore(pgStore, nil, nil, pgStore)
			slog.Warn("Using Postgres without Redis; idempotency and rate limiting fall back to Postgres tables")
		}
	case "badger":
		dbPath := strings.TrimSpace(os.Getenv("VELARIX_BADGER_PATH"))
		if dbPath == "" {
			dbPath = "velarix.data"
		}
		localStore, err := store.OpenBadger(dbPath, encryptionKey)
		if err != nil {
			slog.Error("Failed to open BadgerDB", "error", err)
			os.Exit(1)
		}
		runtimeStore = localStore
		slog.Info("Using local Badger adapter for storage", "mode", "development/test")
	default:
		slog.Error("Unknown store backend", "backend", backend)
		os.Exit(1)
	}

	defer runtimeStore.Close()

	server := &api.Server{
		Engines:    make(map[string]*core.Engine),
		Configs:    make(map[string]*store.SessionConfig),
		Versions:   make(map[string]int64),
		LastAccess: make(map[string]time.Time),
		SliceCache: make(map[string]*api.SliceCacheEntry),
		Store:      runtimeStore,
		StartTime:  time.Now(),
	}

	slog.Info("Session engines will rebuild lazily from shared state", "store_backend", backend)

	server.StartEvictionTicker()
	server.StartRetentionTicker()
	if backend == "badger" {
		server.StartBackupTicker()
	}
	runtimeStore.StartGC()

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
