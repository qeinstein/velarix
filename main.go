package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
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
)

func main() {
	_ = godotenv.Load()

	liteFlag := flag.Bool("lite", false, "Run in Lite mode (no auth, local storage)")
	flag.Parse()

	isLite := *liteFlag || os.Getenv("VELARIX_LITE") == "true"

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

	slog.Info("Velarix | Epistemic Layer for AI Agents", "lite_mode", isLite)

	encryptionKey := []byte(os.Getenv("VELARIX_ENCRYPTION_KEY"))
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_ENV")))
	if env == "" {
		env = "prod"
	}
	devLike := env == "dev" || env == "test"

	if !isLite && len(encryptionKey) == 0 {
		if !devLike {
			slog.Error("CRITICAL: Encryption at rest is REQUIRED in production.")
			os.Exit(1)
		}
	}
	if !isLite && !devLike && strings.TrimSpace(os.Getenv("VELARIX_JWT_SECRET")) == "" {
		slog.Error("CRITICAL: VELARIX_JWT_SECRET is REQUIRED in production.")
		os.Exit(1)
	}

	backend := "badger"
	if !isLite {
		backend = strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_STORE_BACKEND")))
		if backend == "" && strings.TrimSpace(os.Getenv("VELARIX_POSTGRES_DSN")) != "" {
			backend = "postgres"
		}
	}
	postgresDSN := strings.TrimSpace(os.Getenv("VELARIX_POSTGRES_DSN"))
	allowBadgerProd := strings.EqualFold(strings.TrimSpace(os.Getenv("VELARIX_ALLOW_BADGER_PROD")), "true")
	if !isLite && !devLike {
		switch backend {
		case "", "postgres":
			if postgresDSN == "" && !allowBadgerProd {
				slog.Error("CRITICAL: production requires Postgres or VELARIX_ALLOW_BADGER_PROD=true.")
				os.Exit(1)
			}
		case "badger":
			if !allowBadgerProd {
				slog.Error("CRITICAL: production Badger fallback is disabled by default. Set VELARIX_ALLOW_BADGER_PROD=true to opt in.")
				os.Exit(1)
			}
		}
	}

	var runtimeStore store.RuntimeStore
	var primaryStore store.ServerStore
	var compositeClosers []store.RuntimeCloser
	if backend == "postgres" && !isLite {
		pgStore, err := storepostgres.Open(context.Background(), postgresDSN)
		if err != nil {
			slog.Error("Failed to open Postgres store", "error", err)
			os.Exit(1)
		}
		primaryStore = pgStore
		compositeClosers = append(compositeClosers, pgStore)
	} else {
		dbPath := os.Getenv("VELARIX_BADGER_PATH")
		if dbPath == "" {
			dbPath = "velarix.data"
		}
		localStore, err := store.OpenBadger(dbPath, encryptionKey)
		if err != nil {
			slog.Error("Failed to open BadgerDB", "error", err)
			os.Exit(1)
		}
		primaryStore = localStore
		compositeClosers = append(compositeClosers, localStore)
	}

	if !isLite {
		redisURL := strings.TrimSpace(os.Getenv("VELARIX_REDIS_URL"))
		disableRedis := strings.EqualFold(strings.TrimSpace(os.Getenv("VELARIX_DISABLE_REDIS")), "true")

		if redisURL != "" && !disableRedis {
			redisStore, err := storeredis.Open(redisURL)
			if err != nil {
				// Redis ping failed inside Open(). Fallback to primary store for
				// idempotency and rate-limiting rather than hard-exiting.
				slog.Error("Failed to connect to Redis coordination store — falling back to primary store",
					"redis_url", redisURL, "error", err)
				slog.Info("Redis fallback active: idempotency and rate-limiting served by primary store")
				// runtimeStore will be assigned below from primaryStore.
			} else {
				slog.Info("Redis coordination store connected", "redis_url", redisURL)
				compositeClosers = append(compositeClosers, redisStore)
				runtimeStore = store.NewCompositeRuntimeStore(primaryStore, redisStore, redisStore, compositeClosers...)
			}
		} else if disableRedis {
			slog.Info("Redis coordination store disabled (VELARIX_DISABLE_REDIS=true)")
		}
	}
	if runtimeStore == nil {
		if asRuntime, ok := primaryStore.(store.RuntimeStore); ok {
			runtimeStore = asRuntime
		} else {
			runtimeStore = store.NewCompositeRuntimeStore(primaryStore, nil, nil, compositeClosers...)
		}
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
		LiteMode:   isLite,
	}

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

	slog.Info(fmt.Sprintf("Server ready at http://localhost%s", port))

	httpServer := &http.Server{
		Addr:              port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
