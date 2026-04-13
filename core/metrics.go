package core

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// GlobalFanoutTotal counts global truth fan-out operations. "selective" is
	// "true" when the broadcast was limited to a subset of subscribers.
	GlobalFanoutTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "velarix_global_fanout_total",
		Help: "Total GlobalTruth fan-out operations by whether they were selective.",
	}, []string{"selective"})
)
