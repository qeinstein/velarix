package api

import "net/http"

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func traceIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if tid := r.Context().Value(contextKey("trace_id")); tid != nil {
		if traceID, ok := tid.(string); ok {
			return traceID
		}
	}
	return ""
}
