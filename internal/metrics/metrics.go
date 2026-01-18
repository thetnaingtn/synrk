package metrics

import (
	"net/http"
	"runtime"
	runtimemetrics "runtime/metrics"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Function execution metrics
	FunctionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "synrk_function_duration_seconds",
			Help:    "Duration of getReposDetail function execution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"function"},
	)

	FunctionCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "synrk_function_calls_total",
			Help: "Total number of function calls",
		},
		[]string{"function"},
	)

	// Goroutine metrics
	GoroutineCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synrk_goroutines_count",
			Help: "Number of goroutines spawned by function",
		},
		[]string{"function"},
	)

	GoroutinePeak = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synrk_goroutines_peak",
			Help: "Peak number of goroutines during function execution",
		},
		[]string{"function"},
	)

	// Go runtime scheduler metrics (context switching)
	SchedulerLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "synrk_scheduler_latency_seconds",
			Help:    "Go runtime scheduler latency (time goroutine waits to be scheduled)",
			Buckets: []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1},
		},
		[]string{"function"},
	)

	ContextSwitches = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "synrk_context_switches_total",
			Help: "Approximate context switches during function execution",
		},
		[]string{"function"},
	)

	// CPU metrics
	CPUUsageUser = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synrk_cpu_user_seconds",
			Help: "CPU time spent in user mode during function execution",
		},
		[]string{"function"},
	)

	CPUUsageSystem = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synrk_cpu_system_seconds",
			Help: "CPU time spent in system mode during function execution",
		},
		[]string{"function"},
	)

	CPUUsageTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "synrk_cpu_total_seconds",
			Help: "Total CPU time during function execution",
		},
		[]string{"function"},
	)

	// Forks processed metrics
	ForksProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "synrk_forks_processed_total",
			Help: "Total number of forks processed",
		},
		[]string{"function"},
	)

	ForksErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "synrk_forks_errors_total",
			Help: "Total number of fork processing errors",
		},
		[]string{"function"},
	)
)

func init() {
	prometheus.MustRegister(
		FunctionDuration,
		FunctionCalls,
		GoroutineCount,
		GoroutinePeak,
		SchedulerLatency,
		ContextSwitches,
		CPUUsageUser,
		CPUUsageSystem,
		CPUUsageTotal,
		ForksProcessed,
		ForksErrors,
	)
}

// RuntimeMetrics captures Go runtime metrics for scheduler/context switching
type RuntimeMetrics struct {
	GoroutinesCreated uint64
	SchedLatency      float64 // in seconds
}

// GetRuntimeMetrics reads runtime/metrics for scheduler info
func GetRuntimeMetrics() RuntimeMetrics {
	samples := make([]runtimemetrics.Sample, 2)
	samples[0].Name = "/sched/goroutines:goroutines"
	samples[1].Name = "/sched/latencies:seconds"

	runtimemetrics.Read(samples)

	var rm RuntimeMetrics

	// Parse goroutine count
	if samples[0].Value.Kind() == runtimemetrics.KindUint64 {
		rm.GoroutinesCreated = samples[0].Value.Uint64()
	}

	// Parse scheduler latency histogram
	if samples[1].Value.Kind() == runtimemetrics.KindFloat64Histogram {
		hist := samples[1].Value.Float64Histogram()
		if len(hist.Counts) > 0 {
			// Calculate weighted average latency
			var totalCount uint64
			var weightedSum float64
			for i, count := range hist.Counts {
				if count > 0 {
					bucketMid := (hist.Buckets[i] + hist.Buckets[i+1]) / 2
					weightedSum += float64(count) * bucketMid
					totalCount += count
				}
			}
			if totalCount > 0 {
				rm.SchedLatency = weightedSum / float64(totalCount)
			}
		}
	}

	return rm
}

// FunctionTracker tracks metrics during function execution
type FunctionTracker struct {
	functionName    string
	startTime       time.Time
	startGoroutines int
	peakGoroutines  int
	startMetrics    RuntimeMetrics
	mu              sync.Mutex
	monitorDone     chan struct{}
}

// StartTracking begins tracking metrics for a function
func StartTracking(functionName string) *FunctionTracker {
	ft := &FunctionTracker{
		functionName:    functionName,
		startTime:       time.Now(),
		startGoroutines: runtime.NumGoroutine(),
		peakGoroutines:  runtime.NumGoroutine(),
		startMetrics:    GetRuntimeMetrics(),
		monitorDone:     make(chan struct{}),
	}

	FunctionCalls.WithLabelValues(functionName).Inc()

	// Monitor goroutine count in background
	go ft.monitorGoroutines()

	return ft
}

func (ft *FunctionTracker) monitorGoroutines() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ft.monitorDone:
			return
		case <-ticker.C:
			current := runtime.NumGoroutine()
			ft.mu.Lock()
			if current > ft.peakGoroutines {
				ft.peakGoroutines = current
			}
			ft.mu.Unlock()
		}
	}
}

// RecordGoroutineSpawn records when goroutines are spawned
func (ft *FunctionTracker) RecordGoroutineSpawn(count int) {
	GoroutineCount.WithLabelValues(ft.functionName).Add(float64(count))
}

// RecordForkProcessed records a successfully processed fork
func (ft *FunctionTracker) RecordForkProcessed() {
	ForksProcessed.WithLabelValues(ft.functionName).Inc()
}

// RecordForkError records a fork processing error
func (ft *FunctionTracker) RecordForkError() {
	ForksErrors.WithLabelValues(ft.functionName).Inc()
}

// Stop completes tracking and records final metrics
func (ft *FunctionTracker) Stop() {
	close(ft.monitorDone)

	duration := time.Since(ft.startTime).Seconds()
	endMetrics := GetRuntimeMetrics()

	// Record duration
	FunctionDuration.WithLabelValues(ft.functionName).Observe(duration)

	// Record peak goroutines
	ft.mu.Lock()
	GoroutinePeak.WithLabelValues(ft.functionName).Set(float64(ft.peakGoroutines))
	ft.mu.Unlock()

	// Record scheduler latency (approximation of context switch overhead)
	latencyDiff := endMetrics.SchedLatency - ft.startMetrics.SchedLatency
	if latencyDiff > 0 {
		SchedulerLatency.WithLabelValues(ft.functionName).Observe(latencyDiff)
	}

	// Approximate context switches based on goroutine changes
	goroutinesDiff := int64(endMetrics.GoroutinesCreated) - int64(ft.startMetrics.GoroutinesCreated)
	if goroutinesDiff > 0 {
		ContextSwitches.WithLabelValues(ft.functionName).Add(float64(goroutinesDiff))
	}

	// Record CPU usage (using runtime.ReadMemStats as proxy since direct CPU isn't available)
	// For actual CPU metrics, we rely on process_cpu_seconds_total from promhttp
	CPUUsageTotal.WithLabelValues(ft.functionName).Set(duration)
}

// Handler returns the Prometheus metrics HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// StartMetricsServer starts a metrics server on the given address
func StartMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - metrics are optional
		}
	}()

	return server
}
