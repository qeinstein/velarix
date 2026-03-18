package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"velarix/api"
	"velarix/core"
	"velarix/store"
)

func main() {
	log.Println(" Velarix | The Epistemic State Layer for AI Agents")
	
	dbPath := "velarix.data"
	
	encryptionKey := []byte(os.Getenv("VELARIX_ENCRYPTION_KEY"))
	if len(encryptionKey) == 0 {
		log.Println(" [SECURITY] WARNING: Encryption at rest is disabled. Set VELARIX_ENCRYPTION_KEY (16, 24, or 32 bytes) in production.")
	} else {
		log.Printf(" [SECURITY] Encryption at rest enabled (%d-bit AES)", len(encryptionKey)*8)
	}

	badgerStore, err := store.OpenBadger(dbPath, encryptionKey)
	if err != nil {
		log.Fatalf("Failed to open BadgerDB: %v", err)
	}
	defer badgerStore.Close()

	server := &api.Server{
		Engines:   make(map[string]*core.Engine),
		Configs:   make(map[string]*store.SessionConfig),
		LastAccess: make(map[string]time.Time),
		Store:     badgerStore,
		StartTime: time.Now(),
	}

	server.StartEvictionTicker()
	server.Store.StartGC()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	
	log.Printf(" Server ready on http://localhost%s", port)
	log.Println("  Console UI: Run 'cd console && npm run dev' to visualize")
	
	if err := http.ListenAndServe(port, server.Routes()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
