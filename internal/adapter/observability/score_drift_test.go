package observability_test

import (
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/stretchr/testify/assert"
)

func TestScoreDriftMonitor_NewScoreDriftMonitor(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 10, 0.15)

	// Test that the monitor was created successfully by checking its behavior
	// We can't access unexported fields, so we test through public methods
	baseline, exists := sdm.GetBaseline("test_metric")
	assert.False(t, exists)
	assert.Equal(t, 0.0, baseline)

	recentScores := sdm.GetRecentScores("test_metric")
	assert.Empty(t, recentScores)
}

func TestScoreDriftMonitor_UpdateBaseline(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 10, 0.15)

	sdm.UpdateBaseline("accuracy", 0.85)

	baseline, exists := sdm.GetBaseline("accuracy")
	assert.True(t, exists)
	assert.Equal(t, 0.85, baseline)

	// Non-existent metric
	_, exists = sdm.GetBaseline("nonexistent")
	assert.False(t, exists)
}

func TestScoreDriftMonitor_RecordScore(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baseline
	sdm.UpdateBaseline("accuracy", 0.8)

	// Record scores within threshold
	sdm.RecordScore("accuracy", 0.82)
	sdm.RecordScore("accuracy", 0.81)
	sdm.RecordScore("accuracy", 0.83)

	recent := sdm.GetRecentScores("accuracy")
	assert.Len(t, recent, 3)
	assert.Equal(t, []float64{0.82, 0.81, 0.83}, recent)
}

func TestScoreDriftMonitor_RecordScore_ExceedsWindow(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Record more scores than window size
	sdm.RecordScore("accuracy", 0.1)
	sdm.RecordScore("accuracy", 0.2)
	sdm.RecordScore("accuracy", 0.3)
	sdm.RecordScore("accuracy", 0.4)
	sdm.RecordScore("accuracy", 0.5)

	recent := sdm.GetRecentScores("accuracy")
	assert.Len(t, recent, 3)
	assert.Equal(t, []float64{0.3, 0.4, 0.5}, recent) // Should keep last 3
}

func TestScoreDriftMonitor_CalculateDrift(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baseline
	sdm.UpdateBaseline("accuracy", 0.8)

	// Record scores with drift
	sdm.RecordScore("accuracy", 0.9) // 0.1 drift
	sdm.RecordScore("accuracy", 0.9) // 0.1 drift
	sdm.RecordScore("accuracy", 0.9) // 0.1 drift

	drift := sdm.GetDrift("accuracy")
	assert.InDelta(t, 0.1, drift, 0.0001)

	// Test negative drift (should be absolute)
	sdm.Reset()
	sdm.UpdateBaseline("accuracy", 0.8)
	sdm.RecordScore("accuracy", 0.7) // -0.1 drift
	sdm.RecordScore("accuracy", 0.7) // -0.1 drift
	sdm.RecordScore("accuracy", 0.7) // -0.1 drift

	drift = sdm.GetDrift("accuracy")
	assert.InDelta(t, 0.1, drift, 0.0001) // Should be absolute
}

func TestScoreDriftMonitor_CalculateDrift_NoBaseline(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Record scores without baseline
	sdm.RecordScore("accuracy", 0.9)
	sdm.RecordScore("accuracy", 0.9)
	sdm.RecordScore("accuracy", 0.9)

	drift := sdm.GetDrift("accuracy")
	assert.Equal(t, 0.0, drift) // Should be 0 when no baseline
}

func TestScoreDriftMonitor_CalculateDrift_NoRecentScores(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baseline but no recent scores
	sdm.UpdateBaseline("accuracy", 0.8)

	drift := sdm.GetDrift("accuracy")
	assert.Equal(t, 0.0, drift) // Should be 0 when no recent scores
}

func TestScoreDriftMonitor_Reset(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Add some data
	sdm.UpdateBaseline("accuracy", 0.8)
	sdm.RecordScore("accuracy", 0.9)

	// Reset
	sdm.Reset()

	// Should be empty
	_, exists := sdm.GetBaseline("accuracy")
	assert.False(t, exists)

	recent := sdm.GetRecentScores("accuracy")
	assert.Empty(t, recent)
}

func TestScoreDriftManager_NewScoreDriftManager(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftManager()
	assert.NotNil(t, sdm)
	assert.Empty(t, sdm.GetAllMonitors())
}

func TestScoreDriftManager_GetOrCreateMonitor(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftManager()

	// Create new monitor
	monitor1 := sdm.GetOrCreateMonitor("test1", "v1.0", "corpus-v1", 10, 0.15)
	assert.NotNil(t, monitor1)

	// Get existing monitor
	monitor2 := sdm.GetOrCreateMonitor("test1", "v2.0", "corpus-v2", 20, 0.25)
	assert.Equal(t, monitor1, monitor2) // Should be the same instance

	// Create another
	monitor3 := sdm.GetOrCreateMonitor("test2", "v1.0", "corpus-v1", 5, 0.1)
	assert.NotEqual(t, monitor1, monitor3)
}

func TestScoreDriftManager_GetMonitor(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftManager()

	// Get non-existent
	monitor, exists := sdm.GetMonitor("nonexistent")
	assert.Nil(t, monitor)
	assert.False(t, exists)

	// Create and get
	sdm.GetOrCreateMonitor("test", "v1.0", "corpus-v1", 10, 0.15)
	monitor, exists = sdm.GetMonitor("test")
	assert.NotNil(t, monitor)
	assert.True(t, exists)
}

func TestScoreDriftManager_GetAllMonitors(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftManager()

	// Empty initially
	all := sdm.GetAllMonitors()
	assert.Empty(t, all)

	// Add some monitors
	sdm.GetOrCreateMonitor("test1", "v1.0", "corpus-v1", 10, 0.15)
	sdm.GetOrCreateMonitor("test2", "v2.0", "corpus-v2", 20, 0.25)

	all = sdm.GetAllMonitors()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "test1")
	assert.Contains(t, all, "test2")
}

func TestScoreDriftManager_ResetAllMonitors(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftManager()

	// Create monitors and add data
	monitor1 := sdm.GetOrCreateMonitor("test1", "v1.0", "corpus-v1", 10, 0.15)
	monitor2 := sdm.GetOrCreateMonitor("test2", "v2.0", "corpus-v2", 20, 0.25)

	monitor1.UpdateBaseline("accuracy", 0.8)
	monitor1.RecordScore("accuracy", 0.9)
	monitor2.UpdateBaseline("precision", 0.7)
	monitor2.RecordScore("precision", 0.8)

	// Reset all
	sdm.ResetAllMonitors()

	// Should be empty
	_, exists1 := monitor1.GetBaseline("accuracy")
	assert.False(t, exists1)
	_, exists2 := monitor2.GetBaseline("precision")
	assert.False(t, exists2)
}

func TestGlobalScoreDriftFunctions(t *testing.T) {
	t.Parallel()

	// Reset global state
	observability.ResetAllScoreDriftMonitors()

	// Test GetScoreDriftMonitor
	monitor := observability.GetScoreDriftMonitor("global-test", "v1.0", "corpus-v1", 10, 0.15)
	assert.NotNil(t, monitor)

	// Test RecordScoreDriftValue
	observability.RecordScoreDriftValue("accuracy", "v1.0", "corpus-v1", 0.85)

	// Test UpdateBaselineScore
	observability.UpdateBaselineScore("accuracy", "v1.0", "corpus-v1", 0.8)

	// Test GetScoreDrift
	drift := observability.GetScoreDrift("accuracy", "v1.0", "corpus-v1")
	assert.GreaterOrEqual(t, drift, 0.0)

	// Test ResetScoreDriftMonitor
	observability.ResetScoreDriftMonitor("accuracy", "v1.0", "corpus-v1")

	// Test ResetAllScoreDriftMonitors
	observability.ResetAllScoreDriftMonitors()
}

func TestScoreDriftMonitor_DriftDetection(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baseline
	sdm.UpdateBaseline("accuracy", 0.8)

	// Record scores that should trigger drift detection
	sdm.RecordScore("accuracy", 0.95) // 0.15 drift - should trigger
	sdm.RecordScore("accuracy", 0.95) // 0.15 drift - should trigger
	sdm.RecordScore("accuracy", 0.95) // 0.15 drift - should trigger

	// The drift should be detected and recorded
	drift := sdm.GetDrift("accuracy")
	assert.InDelta(t, 0.15, drift, 0.0001)
}

func TestScoreDriftMonitor_NoDriftDetection(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baseline
	sdm.UpdateBaseline("accuracy", 0.8)

	// Record scores within threshold
	sdm.RecordScore("accuracy", 0.82) // 0.02 drift - should not trigger
	sdm.RecordScore("accuracy", 0.83) // 0.03 drift - should not trigger
	sdm.RecordScore("accuracy", 0.84) // 0.04 drift - should not trigger

	// The drift should be within threshold
	drift := sdm.GetDrift("accuracy")
	assert.Less(t, drift, 0.1)
}

func TestScoreDriftMonitor_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 10, 0.15)

	// Set baseline
	sdm.UpdateBaseline("accuracy", 0.8)

	// Run concurrent operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(score float64) {
			sdm.RecordScore("accuracy", score)
			done <- true
		}(0.8 + float64(i)*0.01)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// State should be consistent
	recent := sdm.GetRecentScores("accuracy")
	assert.Len(t, recent, 10)
}

func TestScoreDriftMonitor_MultipleMetrics(t *testing.T) {
	t.Parallel()

	sdm := observability.NewScoreDriftMonitor("v1.0", "corpus-v1", 3, 0.1)

	// Set baselines for multiple metrics
	sdm.UpdateBaseline("accuracy", 0.8)
	sdm.UpdateBaseline("precision", 0.7)
	sdm.UpdateBaseline("recall", 0.6)

	// Record scores for different metrics
	sdm.RecordScore("accuracy", 0.85)
	sdm.RecordScore("precision", 0.75)
	sdm.RecordScore("recall", 0.65)

	// Check each metric
	accDrift := sdm.GetDrift("accuracy")
	precDrift := sdm.GetDrift("precision")
	recDrift := sdm.GetDrift("recall")

	assert.InDelta(t, 0.05, accDrift, 0.0001)
	assert.InDelta(t, 0.05, precDrift, 0.0001)
	assert.InDelta(t, 0.05, recDrift, 0.0001)

	// Check recent scores
	accRecent := sdm.GetRecentScores("accuracy")
	precRecent := sdm.GetRecentScores("precision")
	recRecent := sdm.GetRecentScores("recall")

	assert.Equal(t, []float64{0.85}, accRecent)
	assert.Equal(t, []float64{0.75}, precRecent)
	assert.Equal(t, []float64{0.65}, recRecent)
}
