/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics provides Prometheus metrics for the RBAC operator.
// It tracks reconciliation performance, resource management, and error rates
// to provide comprehensive observability for RBAC operations.
package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Reconciliation metrics
	ReconciliationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_reconciliation_total",
			Help: "Total number of reconciliations by config and result",
		},
		[]string{"config", "controller", "result"}, // result: success/error
	)

	ReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rbac_operator_reconciliation_duration_seconds",
			Help:    "Duration of reconciliation operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"config", "controller"},
	)

	ReconciliationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_reconciliation_errors_total",
			Help: "Total reconciliation errors by type",
		},
		[]string{"config", "controller", "error_type"}, // error_type: validation/template/api/conflict
	)

	// Resource management metrics
	ManagedResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rbac_operator_managed_resources_total",
			Help: "Current number of managed RBAC resources",
		},
		[]string{"config", "resource_type", "namespace"}, // resource_type: role/clusterrole/rolebinding/clusterrolebinding
	)

	ResourceOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_resource_operations_total",
			Help: "Total RBAC resource operations",
		},
		[]string{"config", "resource_type", "operation", "result"}, // operation: create/update/delete
	)

	TemplateProcessingErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_template_processing_errors_total",
			Help: "Template processing errors by template type",
		},
		[]string{"config", "template_type"}, // template_type: name/labels/annotations/subjects
	)

	// Namespace metrics
	ManagedNamespaces = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rbac_operator_managed_namespaces_total",
			Help: "Number of namespaces managed by each config",
		},
		[]string{"config"},
	)

	ActiveConfigs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rbac_operator_namespace_configs_total",
			Help: "Number of active NamespaceRBACConfig resources",
		},
	)

	// Health and performance metrics
	LastSuccessfulReconcile = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rbac_operator_last_successful_reconcile_timestamp",
			Help: "Timestamp of last successful reconciliation",
		},
		[]string{"config", "controller"},
	)

	ConflictResolution = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_conflict_resolution_total",
			Help: "Conflict resolution operations by strategy",
		},
		[]string{"config", "strategy", "resource_type"}, // strategy: merge/replace/ignore
	)

	// Template engine metrics
	TemplateProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rbac_operator_template_processing_duration_seconds",
			Help:    "Duration of template processing operations",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"config", "template_type"},
	)

	// Cleanup metrics
	CleanupOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rbac_operator_cleanup_operations_total",
			Help: "Cleanup operations for orphaned resources",
		},
		[]string{"resource_type", "result"},
	)

	// Health metrics
	OperatorHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rbac_operator_health_status",
			Help: "Operator health status (1=healthy, 0=unhealthy)",
		},
		[]string{"component"}, // component: reconciler/rbac_manager/template_engine
	)
)

func init() {
	// Register metrics with controller-runtime
	metrics.Registry.MustRegister(
		ReconciliationTotal,
		ReconciliationDuration,
		ReconciliationErrors,
		ManagedResources,
		ResourceOperations,
		TemplateProcessingErrors,
		ManagedNamespaces,
		ActiveConfigs,
		LastSuccessfulReconcile,
		ConflictResolution,
		TemplateProcessingDuration,
		CleanupOperations,
		OperatorHealth,
	)
}

// Helper functions for recording metrics

// RecordReconciliation records reconciliation metrics with error categorization
func RecordReconciliation(config, controller string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
		// Categorize error types
		errorType := categorizeError(err)
		ReconciliationErrors.WithLabelValues(config, controller, errorType).Inc()
	}

	ReconciliationTotal.WithLabelValues(config, controller, result).Inc()
	ReconciliationDuration.WithLabelValues(config, controller).Observe(duration.Seconds())

	if err == nil {
		LastSuccessfulReconcile.WithLabelValues(config, controller).SetToCurrentTime()
	}
}

// RecordResourceOperation records RBAC resource create/update/delete operations
func RecordResourceOperation(config, resourceType, operation string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	ResourceOperations.WithLabelValues(config, resourceType, operation, result).Inc()
}

// RecordTemplateProcessing records template processing metrics
func RecordTemplateProcessing(config, templateType string, duration time.Duration, err error) {
	if err != nil {
		TemplateProcessingErrors.WithLabelValues(config, templateType).Inc()
	}
	TemplateProcessingDuration.WithLabelValues(config, templateType).Observe(duration.Seconds())
}

// UpdateManagedResources updates the count of managed resources
func UpdateManagedResources(config, resourceType, namespace string, count int) {
	ManagedResources.WithLabelValues(config, resourceType, namespace).Set(float64(count))
}

// UpdateManagedNamespaces updates the count of managed namespaces
func UpdateManagedNamespaces(config string, count int) {
	ManagedNamespaces.WithLabelValues(config).Set(float64(count))
}

// RecordConflictResolution records merge strategy usage
func RecordConflictResolution(config, strategy, resourceType string) {
	ConflictResolution.WithLabelValues(config, strategy, resourceType).Inc()
}

// RecordCleanup records cleanup operations
func RecordCleanup(resourceType string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	CleanupOperations.WithLabelValues(resourceType, result).Inc()
}

// SetOperatorHealth sets health status for components
func SetOperatorHealth(component string, healthy bool) {
	value := float64(0)
	if healthy {
		value = 1
	}
	OperatorHealth.WithLabelValues(component).Set(value)
}

// categorizeError categorizes errors for better metrics granularity
func categorizeError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()
	errStrLower := strings.ToLower(errStr)

	// Check for specific error types
	if errors.IsNotFound(err) {
		return "not_found"
	}
	if errors.IsConflict(err) {
		return "conflict"
	}
	if errors.IsTimeout(err) {
		return "timeout"
	}
	if errors.IsUnauthorized(err) {
		return "unauthorized"
	}
	if errors.IsForbidden(err) {
		return "forbidden"
	}

	// Check error message content for categorization
	if strings.Contains(errStrLower, "template") {
		return "template"
	}
	if strings.Contains(errStrLower, "validation") || strings.Contains(errStrLower, "invalid") {
		return "validation"
	}
	if strings.Contains(errStrLower, "regex") {
		return "regex"
	}
	if strings.Contains(errStrLower, "timeout") {
		return "timeout"
	}
	if strings.Contains(errStrLower, "connection") || strings.Contains(errStrLower, "network") {
		return "network"
	}

	return "unknown"
}

// ResetMetrics resets all metrics (useful for testing)
func ResetMetrics() {
	ReconciliationTotal.Reset()
	ReconciliationDuration.Reset()
	ReconciliationErrors.Reset()
	ManagedResources.Reset()
	ResourceOperations.Reset()
	TemplateProcessingErrors.Reset()
	ManagedNamespaces.Reset()
	ConflictResolution.Reset()
	TemplateProcessingDuration.Reset()
	CleanupOperations.Reset()
	OperatorHealth.Reset()
	// Note: ActiveConfigs and LastSuccessfulReconcile are not resettable
}
