// Package observability provides logging, metrics, and tracing.
//
// It integrates with OpenTelemetry for system monitoring.
// The package provides comprehensive observability features
// including metrics collection, distributed tracing, and logging.
package observability

import (
	"fmt"
	"log/slog"
	"sync"
)

// ScoreDriftMonitor monitors score drift from baseline.
type ScoreDriftMonitor struct {
	baselineScores map[string]float64
	recentScores   map[string][]float64
	windowSize     int
	driftThreshold float64
	mu             sync.RWMutex
	modelVersion   string
	corpusVersion  string
}

// NewScoreDriftMonitor creates a new score drift monitor.
func NewScoreDriftMonitor(modelVersion, corpusVersion string, windowSize int, driftThreshold float64) *ScoreDriftMonitor {
	return &ScoreDriftMonitor{
		baselineScores: make(map[string]float64),
		recentScores:   make(map[string][]float64),
		windowSize:     windowSize,
		driftThreshold: driftThreshold,
		modelVersion:   modelVersion,
		corpusVersion:  corpusVersion,
	}
}

// UpdateBaseline updates the baseline scores for drift detection.
func (sdm *ScoreDriftMonitor) UpdateBaseline(metricType string, score float64) {
	sdm.mu.Lock()
	defer sdm.mu.Unlock()

	sdm.baselineScores[metricType] = score
	slog.Info("updated baseline score",
		slog.String("metric_type", metricType),
		slog.Float64("score", score),
		slog.String("model_version", sdm.modelVersion),
		slog.String("corpus_version", sdm.corpusVersion))
}

// RecordScore records a new score and checks for drift.
func (sdm *ScoreDriftMonitor) RecordScore(metricType string, score float64) {
	sdm.mu.Lock()
	defer sdm.mu.Unlock()

	// Add to recent scores
	if sdm.recentScores[metricType] == nil {
		sdm.recentScores[metricType] = make([]float64, 0, sdm.windowSize)
	}

	sdm.recentScores[metricType] = append(sdm.recentScores[metricType], score)

	// Maintain window size
	if len(sdm.recentScores[metricType]) > sdm.windowSize {
		sdm.recentScores[metricType] = sdm.recentScores[metricType][1:]
	}

	// Check for drift if we have enough data
	if len(sdm.recentScores[metricType]) >= sdm.windowSize {
		drift := sdm.calculateDrift(metricType)
		if drift > sdm.driftThreshold {
			slog.Warn("score drift detected",
				slog.String("metric_type", metricType),
				slog.Float64("drift", drift),
				slog.Float64("threshold", sdm.driftThreshold),
				slog.String("model_version", sdm.modelVersion),
				slog.String("corpus_version", sdm.corpusVersion))

			// Record drift metric
			RecordScoreDrift(metricType, sdm.modelVersion, sdm.corpusVersion, drift)
		}
	}
}

// calculateDrift calculates the drift from baseline.
func (sdm *ScoreDriftMonitor) calculateDrift(metricType string) float64 {
	baseline, exists := sdm.baselineScores[metricType]
	if !exists {
		return 0.0
	}

	recentScores := sdm.recentScores[metricType]
	if len(recentScores) == 0 {
		return 0.0
	}

	// Calculate average of recent scores
	avgRecent := 0.0
	for _, score := range recentScores {
		avgRecent += score
	}
	avgRecent /= float64(len(recentScores))

	// Calculate drift as absolute difference
	drift := avgRecent - baseline
	if drift < 0 {
		drift = -drift // Make it absolute
	}

	return drift
}

// GetDrift returns the current drift for a metric type.
func (sdm *ScoreDriftMonitor) GetDrift(metricType string) float64 {
	sdm.mu.RLock()
	defer sdm.mu.RUnlock()

	return sdm.calculateDrift(metricType)
}

// GetBaseline returns the baseline score for a metric type.
func (sdm *ScoreDriftMonitor) GetBaseline(metricType string) (float64, bool) {
	sdm.mu.RLock()
	defer sdm.mu.RUnlock()

	score, exists := sdm.baselineScores[metricType]
	return score, exists
}

// GetRecentScores returns the recent scores for a metric type.
func (sdm *ScoreDriftMonitor) GetRecentScores(metricType string) []float64 {
	sdm.mu.RLock()
	defer sdm.mu.RUnlock()

	scores := make([]float64, len(sdm.recentScores[metricType]))
	copy(scores, sdm.recentScores[metricType])
	return scores
}

// Reset resets the drift monitor.
func (sdm *ScoreDriftMonitor) Reset() {
	sdm.mu.Lock()
	defer sdm.mu.Unlock()

	sdm.baselineScores = make(map[string]float64)
	sdm.recentScores = make(map[string][]float64)
}

// ScoreDriftManager manages multiple score drift monitors.
type ScoreDriftManager struct {
	monitors map[string]*ScoreDriftMonitor
	mu       sync.RWMutex
}

// NewScoreDriftManager creates a new score drift manager.
func NewScoreDriftManager() *ScoreDriftManager {
	return &ScoreDriftManager{
		monitors: make(map[string]*ScoreDriftMonitor),
	}
}

// GetOrCreateMonitor gets an existing monitor or creates a new one.
func (sdm *ScoreDriftManager) GetOrCreateMonitor(key, modelVersion, corpusVersion string, windowSize int, driftThreshold float64) *ScoreDriftMonitor {
	sdm.mu.Lock()
	defer sdm.mu.Unlock()

	if monitor, exists := sdm.monitors[key]; exists {
		return monitor
	}

	monitor := NewScoreDriftMonitor(modelVersion, corpusVersion, windowSize, driftThreshold)
	sdm.monitors[key] = monitor
	return monitor
}

// GetMonitor gets an existing monitor.
func (sdm *ScoreDriftManager) GetMonitor(key string) (*ScoreDriftMonitor, bool) {
	sdm.mu.RLock()
	defer sdm.mu.RUnlock()

	monitor, exists := sdm.monitors[key]
	return monitor, exists
}

// GetAllMonitors returns all monitors.
func (sdm *ScoreDriftManager) GetAllMonitors() map[string]*ScoreDriftMonitor {
	sdm.mu.RLock()
	defer sdm.mu.RUnlock()

	result := make(map[string]*ScoreDriftMonitor)
	for key, monitor := range sdm.monitors {
		result[key] = monitor
	}
	return result
}

// ResetAllMonitors resets all monitors.
func (sdm *ScoreDriftManager) ResetAllMonitors() {
	sdm.mu.Lock()
	defer sdm.mu.Unlock()

	for _, monitor := range sdm.monitors {
		monitor.Reset()
	}
}

// Global score drift manager instance
var globalSDM = NewScoreDriftManager()

// GetScoreDriftMonitor gets or creates a score drift monitor.
func GetScoreDriftMonitor(key, modelVersion, corpusVersion string, windowSize int, driftThreshold float64) *ScoreDriftMonitor {
	return globalSDM.GetOrCreateMonitor(key, modelVersion, corpusVersion, windowSize, driftThreshold)
}

// RecordScoreDriftValue records a score for drift monitoring.
func RecordScoreDriftValue(metricType, modelVersion, corpusVersion string, score float64) {
	key := fmt.Sprintf("%s_%s_%s", metricType, modelVersion, corpusVersion)
	monitor := GetScoreDriftMonitor(key, modelVersion, corpusVersion, 10, 0.15) // Default: 10 samples, 0.15 threshold
	monitor.RecordScore(metricType, score)
}

// UpdateBaselineScore updates the baseline score for drift monitoring.
func UpdateBaselineScore(metricType, modelVersion, corpusVersion string, score float64) {
	key := fmt.Sprintf("%s_%s_%s", metricType, modelVersion, corpusVersion)
	monitor := GetScoreDriftMonitor(key, modelVersion, corpusVersion, 10, 0.15)
	monitor.UpdateBaseline(metricType, score)
}

// GetScoreDrift returns the current drift for a metric.
func GetScoreDrift(metricType, modelVersion, corpusVersion string) float64 {
	key := fmt.Sprintf("%s_%s_%s", metricType, modelVersion, corpusVersion)
	monitor, exists := globalSDM.GetMonitor(key)
	if !exists {
		return 0.0
	}
	return monitor.GetDrift(metricType)
}

// ResetScoreDriftMonitor resets a score drift monitor.
func ResetScoreDriftMonitor(metricType, modelVersion, corpusVersion string) {
	key := fmt.Sprintf("%s_%s_%s", metricType, modelVersion, corpusVersion)
	monitor, exists := globalSDM.GetMonitor(key)
	if exists {
		monitor.Reset()
	}
}

// ResetAllScoreDriftMonitors resets all score drift monitors.
func ResetAllScoreDriftMonitors() {
	globalSDM.ResetAllMonitors()
}
