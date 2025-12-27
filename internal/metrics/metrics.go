package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts total requests by provider, agent role, and status
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"provider", "agent_role", "status"},
	)

	// RequestDuration tracks request latency by provider and agent role
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_router_request_duration_seconds",
			Help:    "Request latency distribution",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"provider", "agent_role"},
	)

	// ProviderAvailable indicates if a provider is currently available
	ProviderAvailable = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llm_router_provider_available",
			Help: "Whether a provider is currently available (1=yes, 0=no)",
		},
		[]string{"provider"},
	)

	// ProviderFailures counts provider failures
	ProviderFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_provider_failures_total",
			Help: "Total number of provider failures",
		},
		[]string{"provider"},
	)

	// CacheHits counts cache hits by agent role
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"agent_role"},
	)

	// TokensSaved counts tokens saved due to caching
	TokensSaved = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_tokens_saved_total",
			Help: "Total tokens saved due to prompt caching",
		},
		[]string{"agent_role"},
	)

	// RoutingDecisions tracks routing decisions
	RoutingDecisions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_router_routing_decisions_total",
			Help: "Routing decisions by reason",
		},
		[]string{"reason", "selected_provider"},
	)

	// ActiveRequests tracks in-flight requests
	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llm_router_active_requests",
			Help: "Number of currently active requests",
		},
		[]string{"provider"},
	)

	// ConfidenceDistribution tracks model confidence scores
	ConfidenceDistribution = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_router_confidence",
			Help:    "Distribution of model confidence scores",
			Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
		[]string{"provider", "agent_role"},
	)
)

// RecordRequest records a completed request
func RecordRequest(provider, agentRole, status string, durationSec float64) {
	RequestsTotal.WithLabelValues(provider, agentRole, status).Inc()
	RequestDuration.WithLabelValues(provider, agentRole).Observe(durationSec)
}

// RecordProviderStatus updates provider availability
func RecordProviderStatus(provider string, available bool) {
	val := 0.0
	if available {
		val = 1.0
	}
	ProviderAvailable.WithLabelValues(provider).Set(val)
}

// RecordProviderFailure increments failure count
func RecordProviderFailure(provider string) {
	ProviderFailures.WithLabelValues(provider).Inc()
}

// RecordCacheHit records a cache hit
func RecordCacheHit(agentRole string, tokensSaved int) {
	CacheHits.WithLabelValues(agentRole).Inc()
	TokensSaved.WithLabelValues(agentRole).Add(float64(tokensSaved))
}

// RecordRoutingDecision records why a provider was selected
func RecordRoutingDecision(reason, provider string) {
	RoutingDecisions.WithLabelValues(reason, provider).Inc()
}

// RecordConfidence records model confidence
func RecordConfidence(provider, agentRole string, confidence float64) {
	ConfidenceDistribution.WithLabelValues(provider, agentRole).Observe(confidence)
}
