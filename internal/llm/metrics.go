package llm

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	InferenceLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "llm_inference_latency_seconds",
		Help: "Latency of LLM inference calls.",
	}, []string{"provider", "model"})
)
