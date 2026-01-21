package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
)

type Metrics struct {
	mu sync.RWMutex

	// Counters
	collectionsTotal    map[string]int64
	collectionErrors    map[string]int64
	scalingEventsTotal  map[string]map[string]int64 // cluster -> action -> count
	decisionsTotal      map[string]map[string]int64 // cluster -> decision -> count

	// Gauges
	clusterServerCount  map[string]int
	clusterCPU          map[string]float64
	clusterMemory       map[string]float64
	circuitBreakerState map[string]int // 0=closed, 1=open, 2=half-open

	// Histograms (simplified - just track last values)
	collectionLatency map[string]time.Duration
	decisionLatency   map[string]time.Duration
}

var (
	instance *Metrics
	once     sync.Once
)

func Get() *Metrics {
	once.Do(func() {
		instance = &Metrics{
			collectionsTotal:    make(map[string]int64),
			collectionErrors:    make(map[string]int64),
			scalingEventsTotal:  make(map[string]map[string]int64),
			decisionsTotal:      make(map[string]map[string]int64),
			clusterServerCount:  make(map[string]int),
			clusterCPU:           make(map[string]float64),
			clusterMemory:       make(map[string]float64),
			circuitBreakerState: make(map[string]int),
			collectionLatency:   make(map[string]time.Duration),
			decisionLatency:     make(map[string]time.Duration),
		}
	})
	return instance
}

func (m *Metrics) IncCollections(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collectionsTotal[clusterID]++
}

func (m *Metrics) IncCollectionErrors(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collectionErrors[clusterID]++
}

func (m *Metrics) IncScalingEvent(clusterID, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scalingEventsTotal[clusterID] == nil {
		m.scalingEventsTotal[clusterID] = make(map[string]int64)
	}
	m.scalingEventsTotal[clusterID][action]++
}

func (m *Metrics) IncDecision(clusterID, decision string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.decisionsTotal[clusterID] == nil {
		m.decisionsTotal[clusterID] = make(map[string]int64)
	}
	m.decisionsTotal[clusterID][decision]++
}

func (m *Metrics) SetServerCount(clusterID string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterServerCount[clusterID] = count
}

func (m *Metrics) SetCPU(clusterID string, cpu float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterCPU[clusterID] = cpu
}

func (m *Metrics) SetMemory(clusterID string, memory float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterMemory[clusterID] = memory
}

func (m *Metrics) SetCircuitBreakerState(name string, state int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitBreakerState[name] = state
}

func (m *Metrics) SetCollectionLatency(clusterID string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collectionLatency[clusterID] = d
}

func (m *Metrics) SetDecisionLatency(clusterID string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisionLatency[clusterID] = d
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		defer m.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Collections total
		for cluster, count := range m.collectionsTotal {
			writeMetric(w, "autoscaler_collections_total", map[string]string{"cluster_id": cluster}, float64(count))
		}

		// Collection errors
		for cluster, count := range m.collectionErrors {
			writeMetric(w, "autoscaler_collection_errors_total", map[string]string{"cluster_id": cluster}, float64(count))
		}

		// Scaling events
		for cluster, actions := range m.scalingEventsTotal {
			for action, count := range actions {
				writeMetric(w, "autoscaler_scaling_events_total", map[string]string{"cluster_id": cluster, "action": action}, float64(count))
			}
		}

		// Decisions
		for cluster, decisions := range m.decisionsTotal {
			for decision, count := range decisions {
				writeMetric(w, "autoscaler_decisions_total", map[string]string{"cluster_id": cluster, "decision":  decision}, float64(count))
			}
		}

		// Server count gauge
		for cluster, count := range m.clusterServerCount {
			writeMetric(w, "autoscaler_cluster_servers", map[string]string{"cluster_id": cluster}, float64(count))
		}

		// CPU gauge
		for cluster, cpu := range m.clusterCPU {
			writeMetric(w, "autoscaler_cluster_cpu_percent", map[string]string{"cluster_id": cluster}, cpu)
		}

		// Memory gauge
		for cluster, memory := range m.clusterMemory {
			writeMetric(w, "autoscaler_cluster_memory_percent", map[string]string{"cluster_id": cluster}, memory)
		}

		// Circuit breaker state
		for name, state := range m.circuitBreakerState {
			writeMetric(w, "autoscaler_circuit_breaker_state", map[string]string{"name": name}, float64(state))
		}

		// Collection latency
		for cluster, latency := range m.collectionLatency {
			writeMetric(w, "autoscaler_collection_latency_ms", map[string]string{"cluster_id": cluster}, float64(latency.Milliseconds()))
		}

		// Decision latency
		for cluster, latency := range m.decisionLatency {
			writeMetric(w, "autoscaler_decision_latency_ms", map[string]string{"cluster_id":  cluster}, float64(latency.Milliseconds()))
		}
	})
}

func writeMetric(w http.ResponseWriter, name string, labels map[string]string, value float64) {
	labelStr := ""
	if len(labels) > 0 {
		labelStr = "{"
		first := true
		for k, v := range labels {
			if ! first {
				labelStr += ","
			}
			labelStr += k + `="` + v + `"`
			first = false
		}
		labelStr += "}"
	}
	w.Write([]byte(name + labelStr + " " + strconv.FormatFloat(value, 'f', -1, 64) + "\n"))
}

func StartServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", Get().Handler())

	addr := ":" + strconv.Itoa(port)
	logger.Infof("Prometheus metrics server listening on %s", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Errorf("Prometheus server error: %v", err)
		}
	}()
}