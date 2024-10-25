package gcpip

import (
	"sync"
	"time"
)

// IPAllocationMetrics tracks metrics for IP allocation attempts
type IPAllocationMetrics struct {
	mu            sync.RWMutex
	AttemptCount  int
	FailureCount  int
	LatencyMs     float64
	ErrorTypes    map[string]int
	RegionStats   map[string]*RegionMetrics
	StartTime     time.Time
	LastAttempt   time.Time
}

// RegionMetrics tracks per-region allocation metrics
type RegionMetrics struct {
	Attempts      int
	Failures      int
	AvgLatencyMs  float64
	QuotaLimit    int
	QuotaUsage    int
	LastErrorTime time.Time
}

// NewIPAllocationMetrics creates a new metrics tracker
func NewIPAllocationMetrics() *IPAllocationMetrics {
	return &IPAllocationMetrics{
		ErrorTypes:  make(map[string]int),
		RegionStats: make(map[string]*RegionMetrics),
		StartTime:   time.Now(),
	}
}

// RecordAttempt records an IP allocation attempt
func (m *IPAllocationMetrics) RecordAttempt(region string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.AttemptCount++
	m.LastAttempt = time.Now()
	
	if _, exists := m.RegionStats[region]; !exists {
		m.RegionStats[region] = &RegionMetrics{}
	}
	m.RegionStats[region].Attempts++
}

// RecordError records an IP allocation failure
func (m *IPAllocationMetrics) RecordError(region, errorType string, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.FailureCount++
	m.ErrorTypes[errorType]++
	m.LatencyMs = (m.LatencyMs*float64(m.AttemptCount-1) + latencyMs) / float64(m.AttemptCount)
	
	if stats, exists := m.RegionStats[region]; exists {
		stats.Failures++
		stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.Attempts-1) + latencyMs) / float64(stats.Attempts)
		stats.LastErrorTime = time.Now()
	}
}

// UpdateQuota updates quota information for a region
func (m *IPAllocationMetrics) UpdateQuota(region string, used, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.RegionStats[region]; !exists {
		m.RegionStats[region] = &RegionMetrics{}
	}
	m.RegionStats[region].QuotaUsage = used
	m.RegionStats[region].QuotaLimit = limit
}
