package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// DefaultLatencyBucketsMs covers sub-millisecond up to SLA ceiling (50ms) and breach thresholds.
var DefaultLatencyBucketsMs = []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500.0, 1000.0}

// DefaultCPMBuckets covers CPM clearing price distributions.
var DefaultCPMBuckets = []float64{0.5, 1.0, 2.0, 3.0, 5.0, 7.5, 10.0, 15.0, 25.0, 50.0}

// Counter is a monotonically increasing cumulative metric.
type Counter struct {
	mu    sync.RWMutex
	value float64
}

func (c *Counter) Inc() {
	c.Add(1.0)
}

func (c *Counter) Add(val float64) {
	if val < 0 {
		return
	}
	c.mu.Lock()
	c.value += val
	c.mu.Unlock()
}

func (c *Counter) Get() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// Gauge represents a single numerical value that can arbitrarily go up and down.
type Gauge struct {
	mu    sync.RWMutex
	value float64
}

func (g *Gauge) Set(val float64) {
	g.mu.Lock()
	g.value = val
	g.mu.Unlock()
}

func (g *Gauge) Inc() {
	g.Add(1.0)
}

func (g *Gauge) Dec() {
	g.Add(-1.0)
}

func (g *Gauge) Add(val float64) {
	g.mu.Lock()
	g.value += val
	g.mu.Unlock()
}

func (g *Gauge) Get() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

// Histogram tracks sample observations and calculates percentiles across configurable buckets.
type Histogram struct {
	mu      sync.RWMutex
	buckets []float64
	counts  []int64
	samples []float64 // Stored for exact percentile rendering in demos/scorecards
	count   int64
	sum     float64
	min     float64
	max     float64
}

func NewHistogram(buckets []float64) *Histogram {
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	return &Histogram{
		buckets: sortedBuckets,
		counts:  make([]int64, len(sortedBuckets)),
		samples: make([]float64, 0, 64),
		min:     math.MaxFloat64,
		max:     0,
	}
}

func (h *Histogram) Observe(val float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += val
	if val < h.min {
		h.min = val
	}
	if val > h.max {
		h.max = val
	}
	h.samples = append(h.samples, val)

	for i, b := range h.buckets {
		if val <= b {
			h.counts[i]++
		}
	}
}

func (h *Histogram) GetSummary() HistogramSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return HistogramSummary{}
	}

	sorted := make([]float64, len(h.samples))
	copy(sorted, h.samples)
	sort.Float64s(sorted)

	percentile := func(p float64) float64 {
		if len(sorted) == 0 {
			return 0
		}
		idx := int(float64(len(sorted)-1) * p)
		return sorted[idx]
	}

	minVal := h.min
	if minVal == math.MaxFloat64 {
		minVal = 0
	}

	return HistogramSummary{
		Count:   h.count,
		Sum:     h.sum,
		Average: h.sum / float64(h.count),
		Min:     minVal,
		P50:     percentile(0.50),
		P90:     percentile(0.90),
		P95:     percentile(0.95),
		P99:     percentile(0.99),
		Max:     h.max,
	}
}

// Registry maintains all system metrics and renders them into Prometheus exposition format.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*counterEntry
	gauges     map[string]*gaugeEntry
	histograms map[string]*histogramEntry
}

type counterEntry struct {
	help      string
	labelKeys []string
	instances map[string]*Counter
}

type gaugeEntry struct {
	help      string
	labelKeys []string
	instances map[string]*Gauge
}

type histogramEntry struct {
	help      string
	buckets   []float64
	labelKeys []string
	instances map[string]*Histogram
}

func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*counterEntry),
		gauges:     make(map[string]*gaugeEntry),
		histograms: make(map[string]*histogramEntry),
	}
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return strings.Join(parts, ",")
}

func labelKeyString(labelKeys []string, labels map[string]string) string {
	var parts []string
	for _, k := range labelKeys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return strings.Join(parts, ",")
}

// Counter registration and access
func (r *Registry) RegisterCounter(name, help string, labelKeys ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] = &counterEntry{
		help:      help,
		labelKeys: labelKeys,
		instances: make(map[string]*Counter),
	}
}

func (r *Registry) Counter(name string, labels map[string]string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.counters[name]
	if !ok {
		entry = &counterEntry{
			help:      name,
			instances: make(map[string]*Counter),
		}
		r.counters[name] = entry
	}
	key := labelKeyString(entry.labelKeys, labels)
	c, exists := entry.instances[key]
	if !exists {
		c = &Counter{}
		entry.instances[key] = c
	}
	return c
}

// Gauge registration and access
func (r *Registry) RegisterGauge(name, help string, labelKeys ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[name] = &gaugeEntry{
		help:      help,
		labelKeys: labelKeys,
		instances: make(map[string]*Gauge),
	}
}

func (r *Registry) Gauge(name string, labels map[string]string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.gauges[name]
	if !ok {
		entry = &gaugeEntry{
			help:      name,
			instances: make(map[string]*Gauge),
		}
		r.gauges[name] = entry
	}
	key := labelKeyString(entry.labelKeys, labels)
	g, exists := entry.instances[key]
	if !exists {
		g = &Gauge{}
		entry.instances[key] = g
	}
	return g
}

// Histogram registration and access
func (r *Registry) RegisterHistogram(name, help string, buckets []float64, labelKeys ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(buckets) == 0 {
		buckets = DefaultLatencyBucketsMs
	}
	r.histograms[name] = &histogramEntry{
		help:      help,
		buckets:   buckets,
		labelKeys: labelKeys,
		instances: make(map[string]*Histogram),
	}
}

func (r *Registry) Histogram(name string, labels map[string]string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.histograms[name]
	if !ok {
		entry = &histogramEntry{
			help:      name,
			buckets:   DefaultLatencyBucketsMs,
			instances: make(map[string]*Histogram),
		}
		r.histograms[name] = entry
	}
	key := labelKeyString(entry.labelKeys, labels)
	h, exists := entry.instances[key]
	if !exists {
		h = NewHistogram(entry.buckets)
		entry.instances[key] = h
	}
	return h
}

// RenderPrometheus formats all metrics according to the Prometheus text-based exposition specification.
func (r *Registry) RenderPrometheus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder

	// 1. Counters
	counterNames := make([]string, 0, len(r.counters))
	for name := range r.counters {
		counterNames = append(counterNames, name)
	}
	sort.Strings(counterNames)

	for _, name := range counterNames {
		entry := r.counters[name]
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, entry.help))
		sb.WriteString(fmt.Sprintf("# TYPE %s counter\n", name))

		keys := make([]string, 0, len(entry.instances))
		for k := range entry.instances {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			inst := entry.instances[k]
			if k != "" {
				sb.WriteString(fmt.Sprintf("%s{%s} %g\n", name, k, inst.Get()))
			} else {
				sb.WriteString(fmt.Sprintf("%s %g\n", name, inst.Get()))
			}
		}
	}

	// 2. Gauges
	gaugeNames := make([]string, 0, len(r.gauges))
	for name := range r.gauges {
		gaugeNames = append(gaugeNames, name)
	}
	sort.Strings(gaugeNames)

	for _, name := range gaugeNames {
		entry := r.gauges[name]
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, entry.help))
		sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", name))

		keys := make([]string, 0, len(entry.instances))
		for k := range entry.instances {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			inst := entry.instances[k]
			if k != "" {
				sb.WriteString(fmt.Sprintf("%s{%s} %g\n", name, k, inst.Get()))
			} else {
				sb.WriteString(fmt.Sprintf("%s %g\n", name, inst.Get()))
			}
		}
	}

	// 3. Histograms
	histNames := make([]string, 0, len(r.histograms))
	for name := range r.histograms {
		histNames = append(histNames, name)
	}
	sort.Strings(histNames)

	for _, name := range histNames {
		entry := r.histograms[name]
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, entry.help))
		sb.WriteString(fmt.Sprintf("# TYPE %s histogram\n", name))

		keys := make([]string, 0, len(entry.instances))
		for k := range entry.instances {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			inst := entry.instances[k]
			inst.mu.RLock()

			for i, b := range inst.buckets {
				lbl := fmt.Sprintf("le=\"%g\"", b)
				if k != "" {
					lbl = fmt.Sprintf("%s,%s", k, lbl)
				}
				sb.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", name, lbl, inst.counts[i]))
			}

			infLbl := "le=\"+Inf\""
			if k != "" {
				infLbl = fmt.Sprintf("%s,%s", k, infLbl)
			}
			sb.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", name, infLbl, inst.count))

			sumLbl := ""
			if k != "" {
				sumLbl = fmt.Sprintf("{%s}", k)
			}
			sb.WriteString(fmt.Sprintf("%s_sum%s %g\n", name, sumLbl, inst.sum))
			sb.WriteString(fmt.Sprintf("%s_count%s %d\n", name, sumLbl, inst.count))

			inst.mu.RUnlock()
		}
	}

	return sb.String()
}

// ExportJSON dumps a structured snapshot of metrics.
func (r *Registry) ExportJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data := map[string]interface{}{
		"counters":   make(map[string]map[string]float64),
		"gauges":     make(map[string]map[string]float64),
		"histograms": make(map[string]map[string]HistogramSummary),
	}

	for name, entry := range r.counters {
		m := make(map[string]float64)
		for k, inst := range entry.instances {
			m[k] = inst.Get()
		}
		data["counters"].(map[string]map[string]float64)[name] = m
	}

	for name, entry := range r.gauges {
		m := make(map[string]float64)
		for k, inst := range entry.instances {
			m[k] = inst.Get()
		}
		data["gauges"].(map[string]map[string]float64)[name] = m
	}

	for name, entry := range r.histograms {
		m := make(map[string]HistogramSummary)
		for k, inst := range entry.instances {
			m[k] = inst.GetSummary()
		}
		data["histograms"].(map[string]map[string]HistogramSummary)[name] = m
	}

	return json.MarshalIndent(data, "", "  ")
}
