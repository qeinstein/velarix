package api

import (
	"encoding/json"
	"net/http"

	"causaldb/core"
	"causaldb/store"
)



type Server struct {
	Engine  *core.Engine
	Journal *store.Journal
}


func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}


func (s *Server) handleAssertFact(w http.ResponseWriter, r *http.Request) {
	var fact core.Fact

	if err := json.NewDecoder(r.Body).Decode(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.Journal.AppendAssert(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mutate engine
	if err := s.Engine.AssertFact(&fact); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, fact)
}


func (s *Server) handleInvalidateRoot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.Journal.AppendInvalidate(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.Engine.InvalidateRoot(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
}


func (s *Server) handleGetFact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	fact, ok := s.Engine.Facts[id]
	if !ok {
		http.Error(w, "fact not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, fact)
}


func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	explanations, err := s.Engine.Explain(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, explanations)
}


func (s *Server) handleListFacts(w http.ResponseWriter, r *http.Request) {
	validOnly := r.URL.Query().Get("valid") == "true"

	var result []*core.Fact
	for _, fact := range s.Engine.Facts {
		if validOnly && fact.DerivedStatus != core.Valid {
			continue
		}
		result = append(result, fact)
	}

	writeJSON(w, http.StatusOK, result)
}


func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /facts", s.handleAssertFact)
	mux.HandleFunc("POST /facts/{id}/invalidate", s.handleInvalidateRoot)
	mux.HandleFunc("GET /facts/{id}", s.handleGetFact)
	mux.HandleFunc("GET /facts/{id}/why", s.handleExplain)
	mux.HandleFunc("GET /facts", s.handleListFacts)

	return mux
}
