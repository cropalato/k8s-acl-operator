package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRecordReconciliation(t *testing.T) {
	ResetMetrics()

	// Test successful reconciliation
	RecordReconciliation("test-config", "NamespaceRBACConfig", time.Millisecond*100, nil)

	// Verify metrics
	if got := testutil.ToFloat64(ReconciliationTotal.WithLabelValues("test-config", "NamespaceRBACConfig", "success")); got != 1 {
		t.Errorf("Expected 1 successful reconciliation, got %f", got)
	}

	// Test failed reconciliation
	err := errors.New("template error")
	RecordReconciliation("test-config", "NamespaceRBACConfig", time.Millisecond*200, err)

	if got := testutil.ToFloat64(ReconciliationTotal.WithLabelValues("test-config", "NamespaceRBACConfig", "error")); got != 1 {
		t.Errorf("Expected 1 failed reconciliation, got %f", got)
	}

	if got := testutil.ToFloat64(ReconciliationErrors.WithLabelValues("test-config", "NamespaceRBACConfig", "template")); got != 1 {
		t.Errorf("Expected 1 template error, got %f", got)
	}
}

func TestRecordResourceOperation(t *testing.T) {
	ResetMetrics()

	RecordResourceOperation("test-config", "role", "create", nil)
	RecordResourceOperation("test-config", "role", "update", errors.New("api error"))

	if got := testutil.ToFloat64(ResourceOperations.WithLabelValues("test-config", "role", "create", "success")); got != 1 {
		t.Errorf("Expected 1 successful create, got %f", got)
	}

	if got := testutil.ToFloat64(ResourceOperations.WithLabelValues("test-config", "role", "update", "error")); got != 1 {
		t.Errorf("Expected 1 failed update, got %f", got)
	}
}

func TestRecordTemplateProcessing(t *testing.T) {
	ResetMetrics()

	RecordTemplateProcessing("test-config", "role_name", time.Millisecond*10, nil)
	RecordTemplateProcessing("test-config", "role_name", time.Millisecond*5, errors.New("template parse error"))

	if got := testutil.ToFloat64(TemplateProcessingErrors.WithLabelValues("test-config", "role_name")); got != 1 {
		t.Errorf("Expected 1 template error, got %f", got)
	}
}

func TestUpdateManagedResources(t *testing.T) {
	ResetMetrics()

	UpdateManagedResources("test-config", "role", "test-ns", 5)

	if got := testutil.ToFloat64(ManagedResources.WithLabelValues("test-config", "role", "test-ns")); got != 5 {
		t.Errorf("Expected 5 managed resources, got %f", got)
	}
}

func TestUpdateManagedNamespaces(t *testing.T) {
	ResetMetrics()

	UpdateManagedNamespaces("test-config", 3)

	if got := testutil.ToFloat64(ManagedNamespaces.WithLabelValues("test-config")); got != 3 {
		t.Errorf("Expected 3 managed namespaces, got %f", got)
	}
}

func TestRecordConflictResolution(t *testing.T) {
	ResetMetrics()

	RecordConflictResolution("test-config", "merge", "role")
	RecordConflictResolution("test-config", "replace", "clusterrole")

	if got := testutil.ToFloat64(ConflictResolution.WithLabelValues("test-config", "merge", "role")); got != 1 {
		t.Errorf("Expected 1 merge operation, got %f", got)
	}

	if got := testutil.ToFloat64(ConflictResolution.WithLabelValues("test-config", "replace", "clusterrole")); got != 1 {
		t.Errorf("Expected 1 replace operation, got %f", got)
	}
}

func TestRecordCleanup(t *testing.T) {
	ResetMetrics()

	RecordCleanup("clusterrole", nil)
	RecordCleanup("clusterrolebinding", errors.New("cleanup failed"))

	if got := testutil.ToFloat64(CleanupOperations.WithLabelValues("clusterrole", "success")); got != 1 {
		t.Errorf("Expected 1 successful cleanup, got %f", got)
	}

	if got := testutil.ToFloat64(CleanupOperations.WithLabelValues("clusterrolebinding", "error")); got != 1 {
		t.Errorf("Expected 1 failed cleanup, got %f", got)
	}
}

func TestSetOperatorHealth(t *testing.T) {
	ResetMetrics()

	SetOperatorHealth("reconciler", true)
	SetOperatorHealth("rbac_manager", false)

	if got := testutil.ToFloat64(OperatorHealth.WithLabelValues("reconciler")); got != 1 {
		t.Errorf("Expected reconciler health = 1, got %f", got)
	}

	if got := testutil.ToFloat64(OperatorHealth.WithLabelValues("rbac_manager")); got != 0 {
		t.Errorf("Expected rbac_manager health = 0, got %f", got)
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{nil, "none"},
		{k8serrors.NewNotFound(schema.GroupResource{}, "test"), "not_found"},
		{k8serrors.NewConflict(schema.GroupResource{}, "test", errors.New("conflict")), "conflict"},
		{k8serrors.NewTimeoutError("timeout", 1), "timeout"},
		{k8serrors.NewUnauthorized("unauthorized"), "unauthorized"},
		{k8serrors.NewForbidden(schema.GroupResource{}, "test", errors.New("forbidden")), "forbidden"},
		{errors.New("template parsing failed"), "template"},
		{errors.New("validation error"), "validation"},
		{errors.New("invalid regex pattern"), "regex"},
		{errors.New("connection refused"), "network"},
		{errors.New("unknown error"), "unknown"},
	}

	for _, tt := range tests {
		got := categorizeError(tt.err)
		if got != tt.expected {
			t.Errorf("categorizeError(%v) = %s, want %s", tt.err, got, tt.expected)
		}
	}
}

func TestResetMetrics(t *testing.T) {
	// Record some metrics
	RecordReconciliation("test", "test", time.Second, nil)
	UpdateManagedNamespaces("test", 5)

	// Reset
	ResetMetrics()

	// Verify counters are reset (gauges might retain values)
	if got := testutil.ToFloat64(ReconciliationTotal.WithLabelValues("test", "test", "success")); got != 0 {
		t.Errorf("Expected counter to be reset to 0, got %f", got)
	}
}
