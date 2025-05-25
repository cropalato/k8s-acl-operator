package health

import (
	"net/http"
	"testing"
	"time"

	"github.com/yourusername/k8s-acl-operator/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestHealthChecker(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	checker := NewChecker(log.Log)

	// Initially healthy but not ready
	if !checker.IsHealthy() {
		t.Error("Should start healthy")
	}
	if checker.IsReady() {
		t.Error("Should not start ready")
	}

	// Set ready
	checker.SetReady(true)
	if !checker.IsReady() {
		t.Error("Should be ready after SetReady(true)")
	}

	// Record reconcile should keep healthy
	checker.RecordReconcile()
	if !checker.IsHealthy() {
		t.Error("Should stay healthy after reconcile")
	}

	// Set unhealthy
	checker.SetHealthy(false)
	if checker.IsHealthy() {
		t.Error("Should be unhealthy after SetHealthy(false)")
	}
}

func TestHealthCheckerProbes(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	checker := NewChecker(log.Log)
	checker.SetReady(true)

	// Test liveness probe
	req, _ := http.NewRequest("GET", "/healthz", nil)
	err := checker.LivenessCheck(req)
	if err != nil {
		t.Errorf("Liveness check should pass when healthy: %v", err)
	}

	// Test readiness probe
	err = checker.ReadinessCheck(req)
	if err != nil {
		t.Errorf("Readiness check should pass when ready and healthy: %v", err)
	}

	// Set unhealthy
	checker.SetHealthy(false)
	err = checker.LivenessCheck(req)
	if err == nil {
		t.Error("Liveness check should fail when unhealthy")
	}

	err = checker.ReadinessCheck(req)
	if err == nil {
		t.Error("Readiness check should fail when unhealthy")
	}
}

func TestHealthCheckerTimeout(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	// This test would normally take 5+ minutes, so we'll test the logic
	checker := NewChecker(log.Log)
	checker.SetReady(true)

	// Simulate old reconcile time
	checker.lastReconcile = time.Now().Add(-6 * time.Minute).Unix()

	if checker.IsHealthy() {
		t.Error("Should be unhealthy after timeout")
	}
}
