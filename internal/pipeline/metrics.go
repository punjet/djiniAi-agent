package pipeline

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	JobApplicationCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "job_application_total",
		Help: "Total number of job applications processed.",
	}, []string{"status"})
)
