package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func requestIDForResponse(w http.ResponseWriter, r *http.Request) string {
	if r != nil {
		if traceID, _ := r.Context().Value(contextKey("trace_id")).(string); traceID != "" {
			if w != nil {
				w.Header().Set("X-Trace-Id", traceID)
			}
			return traceID
		}
		if incoming := r.Header.Get("X-Trace-Id"); incoming != "" {
			if w != nil {
				w.Header().Set("X-Trace-Id", incoming)
			}
			return incoming
		}
	}
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	if w != nil {
		w.Header().Set("X-Trace-Id", requestID)
	}
	return requestID
}

func writeOpaqueError(w http.ResponseWriter, r *http.Request, status int, err error, logMsg string) {
	requestID := requestIDForResponse(w, r)
	if err != nil {
		slog.Error(logMsg, "request_id", requestID, "error", err)
	} else {
		slog.Error(logMsg, "request_id", requestID)
	}
	http.Error(w, "an internal error occurred; request_id="+requestID, status)
}
