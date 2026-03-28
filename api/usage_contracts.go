package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"velarix/store"
)

type timeseriesPoint struct {
	TimestampMs int64  `json:"ts"`
	Value       uint64 `json:"value"`
}

type timeseriesSeries struct {
	Metric string            `json:"metric"`
	Points []timeseriesPoint `json:"points"`
}

func (s *Server) handleGetUsageTimeseries(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)

	metricParam := strings.TrimSpace(r.URL.Query().Get("metric"))
	metrics := []string{}
	if metricParam != "" {
		for _, m := range strings.Split(metricParam, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				metrics = append(metrics, m)
			}
		}
	}
	if len(metrics) == 0 {
		metrics = []string{
			"api_requests",
			"facts_asserted",
			"schema_violations",
			"facts_pruned",
			"sessions_created",
			"revalidation_runs",
		}
	}

	now := time.Now().UnixMilli()
	fromMs := now - 24*60*60*1000
	toMs := now
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			fromMs = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			toMs = parsed
		}
	}

	bucket := strings.TrimSpace(r.URL.Query().Get("bucket"))
	bucketMs := int64(60000)
	switch bucket {
	case "minute", "":
		bucketMs = 60000
	case "hour":
		bucketMs = 60 * 60000
	case "day":
		bucketMs = 24 * 60 * 60000
	default:
		http.Error(w, "invalid bucket (minute|hour|day)", http.StatusBadRequest)
		return
	}

	series := []timeseriesSeries{}
	for _, metric := range metrics {
		points, err := s.Store.GetOrgMetricTimeseries(orgID, metric, fromMs, toMs, bucketMs)
		if err != nil {
			http.Error(w, "failed to read timeseries", http.StatusInternalServerError)
			return
		}
		outPts := make([]timeseriesPoint, 0, len(points))
		for _, p := range points {
			outPts = append(outPts, timeseriesPoint{TimestampMs: p.TimestampMs, Value: p.Value})
		}
		series = append(series, timeseriesSeries{Metric: metric, Points: outPts})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"from":   fromMs,
		"to":     toMs,
		"bucket": bucketMs,
		"series": series,
	})
}

func (s *Server) handleGetUsageBreakdown(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	b, err := s.Store.GetOrgUsageBreakdown(orgID)
	if err != nil {
		http.Error(w, "failed to read breakdown", http.StatusInternalServerError)
		return
	}

	type endpointRow struct {
		Endpoint string `json:"endpoint"`
		Count    uint64 `json:"count"`
	}
	type statusRow struct {
		Status string `json:"status"`
		Count  uint64 `json:"count"`
	}

	byEndpoint := []endpointRow{}
	for ep, v := range b.ByEndpoint {
		byEndpoint = append(byEndpoint, endpointRow{Endpoint: ep, Count: v})
	}
	sort.Slice(byEndpoint, func(i, j int) bool { return byEndpoint[i].Count > byEndpoint[j].Count })

	byStatus := []statusRow{}
	for st, v := range b.ByStatus {
		byStatus = append(byStatus, statusRow{Status: st, Count: v})
	}
	sort.Slice(byStatus, func(i, j int) bool { return byStatus[i].Count > byStatus[j].Count })

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"by_endpoint": byEndpoint,
		"by_status":   byStatus,
		"raw":         b.Raw,
	})
}

var _ = store.MetricPoint{}

