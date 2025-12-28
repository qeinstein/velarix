package main

import (
	"log"
	"net/http"

	"causaldb/api"
	"causaldb/core"
	"causaldb/store"
)

func main() {
	engine := core.NewEngine()

	journal, err := store.OpenJournal("causaldb.log")
	if err != nil {
		log.Fatal(err)
	}

	// Replay persisted state
	if err := store.Replay("causaldb.log", engine); err != nil {
		log.Fatal(err)
	}

	server := &api.Server{
		Engine:  engine,
		Journal: journal,
	}

	log.Println("CausalDB listening on :8080")
	http.ListenAndServe(":8080", server.Routes())
}
