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
	env := os.Getenv("VELARIX_ENV")

	if !isLite && len(encryptionKey) == 0 {
		if env != "dev" {
			slog.Error("CRITICAL: Encryption at rest is REQUIRED in production.")
			os.Exit(1)
		}
	}

	backend := "badger"
	if !isLite {
		backend = strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_STORE_BACKEND")))
		if backend == "" && strings.TrimSpace(os.Getenv("VELARIX_POSTGRES_DSN")) != "" {
			backend = "postgres"
		}
	}

	var runtimeStore store.RuntimeStore
	if backend == "postgres" && !isLite {
		pgStore, err := storepostgres.Open(context.Background(), os.Getenv("VELARIX_POSTGRES_DSN"))
		if err != nil {
			slog.Error("Failed to open Postgres store", "error", err)
			os.Exit(1)
		}
		runtimeStore = store.NewCompositeRuntimeStore(pgStore, nil, nil, pgStore)
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
		runtimeStore = localStore
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

	if err := http.ListenAndServe(port, server.Routes()); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
