package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"velarix/store"
)

type exportJobCreateRequest struct {
	Format string `json:"format"`
}

func exportJobView(sessionID string, job *store.ExportJob) map[string]interface{} {
	resp := map[string]interface{}{
		"id":           job.ID,
		"session_id":   job.SessionID,
		"format":       job.Format,
		"status":       job.Status,
		"error":        job.Error,
		"created_at":   job.CreatedAt,
		"completed_at": job.CompletedAt,
		"size_bytes":   job.SizeBytes,
	}
	if job.Status == "done" {
		resp["download_url"] = "/v1/s/" + sessionID + "/export-jobs/" + job.ID + "/download"
		resp["filename"] = job.Filename
		resp["content_type"] = job.ContentType
	}
	return resp
}

func (s *Server) handleCreateExportJob(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	engine, _, err := s.getEngine(sessionID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var body exportJobCreateRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Format == "" {
		body.Format = "csv"
	}
	switch body.Format {
	case "csv", "pdf":
	default:
		http.Error(w, "unsupported export format (csv|pdf)", http.StatusBadRequest)
		return
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := "exp_" + hex.EncodeToString(b)
	job := &store.ExportJob{
		ID:        id,
		SessionID: sessionID,
		OrgID:     orgID,
		Format:    body.Format,
		Status:    "queued",
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.Store.SaveExportJob(job); err != nil {
		http.Error(w, "failed to persist job", http.StatusInternalServerError)
		return
	}

	go func() {
		job.Status = "running"
		_ = s.Store.SaveExportJob(job)
		history, _ := s.Store.GetSessionHistory(sessionID)
		usage, _ := s.Store.GetOrgUsage(orgID)
		chainHead, _ := s.Store.GetSessionHistoryChainHead(sessionID)
		ct, fn, data, expErr := buildSessionExport(sessionID, orgID, engine, history, usage, chainHead, body.Format)
		if expErr != nil {
			_ = s.Store.SaveExportJobResult(sessionID, id, "", "", nil, expErr.Error())
			return
		}
		_ = s.Store.SaveExportJobResult(sessionID, id, ct, fn, data, "")
	}()

	writeJSON(w, http.StatusCreated, exportJobView(sessionID, job))
}

func (s *Server) handleGetExportJob(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	// Verify org ownership
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	job, err := s.Store.GetExportJob(sessionID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, exportJobView(sessionID, job))
}

func (s *Server) handleDownloadExportJob(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	job, err := s.Store.GetExportJob(sessionID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if job.Status != "done" {
		http.Error(w, "export not ready", http.StatusConflict)
		return
	}
	data, err := s.Store.GetExportJobData(sessionID, id)
	if err != nil {
		http.Error(w, "export data missing", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", job.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+job.Filename)
	_, _ = w.Write(data)

	_ = s.Store.AppendOrgActivity(orgID, store.JournalEntry{
		Type:      store.EventAdminAction,
		SessionID: sessionID,
		ActorID:   getActorID(r),
		Payload:   map[string]interface{}{"action": "export_download", "job_id": id, "filename": job.Filename},
		Timestamp: time.Now().UnixMilli(),
	})
}

func (s *Server) handleListExportJobs(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	orgID := getOrgID(r)
	storedOrg, err := s.Store.GetSessionOrganization(sessionID)
	if err != nil || storedOrg != orgID {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}
	jobs, err := s.Store.ListExportJobs(sessionID, 50)
	if err != nil {
		http.Error(w, "failed to list jobs", http.StatusInternalServerError)
		return
	}
	items := make([]map[string]interface{}, 0, len(jobs))
	for i := range jobs {
		j := jobs[i]
		items = append(items, exportJobView(sessionID, &j))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}
