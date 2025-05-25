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

// Package namespacerbacconfig contains the controller logic for NamespaceRBACConfig resources.
// This controller watches for NamespaceRBACConfig CRDs and automatically creates/manages
// RBAC resources (Roles, ClusterRoles, RoleBindings, ClusterRoleBindings) for matching namespaces.
package namespacerbacconfig

import (
	"context"
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	"github.com/yourusername/k8s-acl-operator/pkg/health"
	"github.com/yourusername/k8s-acl-operator/pkg/metrics"
	"github.com/yourusername/k8s-acl-operator/pkg/rbac"
	"github.com/yourusername/k8s-acl-operator/pkg/utils"
)

const (
	// ConditionTypeReady indicates whether the NamespaceRBACConfig is ready
	// and successfully applying RBAC to matching namespaces
	ConditionTypeReady = "Ready"
	// ConditionTypeProgressing indicates whether the NamespaceRBACConfig is progressing
	// through reconciliation (creating/updating RBAC resources)
	ConditionTypeProgressing = "Progressing"
	// ConditionTypeDegraded indicates whether the NamespaceRBACConfig is degraded
	// due to errors during reconciliation
	ConditionTypeDegraded = "Degraded"

	// ReasonReconcileSuccess indicates successful reconciliation
	ReasonReconcileSuccess = "ReconcileSuccess"
	// ReasonReconcileError indicates reconciliation error
	ReasonReconcileError = "ReconcileError"
	// ReasonValidationError indicates validation error
	ReasonValidationError = "ValidationError"

	// FinalizerName is the finalizer used by this controller to ensure proper cleanup
	// of cluster-scoped resources when the NamespaceRBACConfig is deleted
	FinalizerName = "namespacerbacconfig.rbac.operator.io/finalizer"
)

// NamespaceRBACConfigReconciler reconciles a NamespaceRBACConfig object.
// It watches for changes to NamespaceRBACConfig resources and applies the defined
// RBAC templates to matching namespaces. The reconciler also handles cleanup
// when configs are deleted.
type NamespaceRBACConfigReconciler struct {
	client.Client                 // Kubernetes API client
	Scheme        *runtime.Scheme // Kubernetes scheme for object serialization
	Log           logr.Logger     // Structured logger
	rbacManager   *rbac.Manager   // Handles RBAC resource creation/management
	healthChecker *health.Checker // Health monitoring
}

// NewNamespaceRBACConfigReconciler creates a new reconciler
func NewNamespaceRBACConfigReconciler(client client.Client, scheme *runtime.Scheme, log logr.Logger, healthChecker *health.Checker) *NamespaceRBACConfigReconciler {
	return &NamespaceRBACConfigReconciler{
		Client:        client,
		Scheme:        scheme,
		Log:           log,
		rbacManager:   rbac.NewManager(client),
		healthChecker: healthChecker,
	}
}

// +kubebuilder:rbac:groups=rbac.operator.io,resources=namespacerbacconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.operator.io,resources=namespacerbacconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.operator.io,resources=namespacerbacconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The reconciliation flow:
// 1. Fetch the NamespaceRBACConfig resource
// 2. Handle deletion if the resource is being deleted
// 3. Add finalizer if not present (for proper cleanup)
// 4. Validate the configuration
// 5. Find all namespaces matching the selector
// 6. Apply RBAC templates to matching namespaces
// 7. Update status with results
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *NamespaceRBACConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	start := time.Now()
	log := r.Log.WithValues("namespacerbacconfig", req.NamespacedName)

	// Fetch the NamespaceRBACConfig instance
	config := &rbacoperatorv1.NamespaceRBACConfig{}
	err := r.Get(ctx, req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("NamespaceRBACConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get NamespaceRBACConfig")
		r.healthChecker.SetHealthy(false)
		metrics.SetOperatorHealth("reconciler", false)
		metrics.RecordReconciliation(req.Name, "NamespaceRBACConfig", time.Since(start), err)
		return ctrl.Result{}, err
	}

	// Record active configs count and defer final metrics recording
	defer func() {
		configList := &rbacoperatorv1.NamespaceRBACConfigList{}
		if listErr := r.List(ctx, configList); listErr == nil {
			metrics.ActiveConfigs.Set(float64(len(configList.Items)))
		}
		metrics.RecordReconciliation(config.Name, "NamespaceRBACConfig", time.Since(start), err)
	}()

	// Handle deletion
	if config.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, config, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(config, FinalizerName) {
		controllerutil.AddFinalizer(config, FinalizerName)
		if err := r.Update(ctx, config); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set progressing condition
	r.setCondition(config, ConditionTypeProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling NamespaceRBACConfig")

	// Validate the configuration
	if err := r.validateConfig(config); err != nil {
		log.Error(err, "Invalid configuration")
		r.healthChecker.SetHealthy(false)
		metrics.SetOperatorHealth("reconciler", false)
		r.setCondition(config, ConditionTypeDegraded, metav1.ConditionTrue, ReasonValidationError, err.Error())
		r.setCondition(config, ConditionTypeReady, metav1.ConditionFalse, ReasonValidationError, "Configuration validation failed")
		r.setCondition(config, ConditionTypeProgressing, metav1.ConditionFalse, ReasonValidationError, "Validation failed")
		return r.updateStatus(ctx, config, log)
	}

	// Reconcile RBAC for all matching namespaces
	appliedNamespaces, err := r.reconcileRBAC(ctx, config, log)
	if err != nil {
		log.Error(err, "Failed to reconcile RBAC")
		r.healthChecker.SetHealthy(false)
		metrics.SetOperatorHealth("reconciler", false)
		r.setCondition(config, ConditionTypeDegraded, metav1.ConditionTrue, ReasonReconcileError, err.Error())
		r.setCondition(config, ConditionTypeReady, metav1.ConditionFalse, ReasonReconcileError, "RBAC reconciliation failed")
		r.setCondition(config, ConditionTypeProgressing, metav1.ConditionFalse, ReasonReconcileError, "Reconciliation failed")
		return r.updateStatus(ctx, config, log)
	}

	// Update status
	config.Status.AppliedNamespaces = appliedNamespaces
	config.Status.ObservedGeneration = config.Generation

	// Update managed namespaces metric
	metrics.UpdateManagedNamespaces(config.Name, len(appliedNamespaces))

	// Set success conditions
	r.healthChecker.RecordReconcile()
	metrics.SetOperatorHealth("reconciler", true)
	r.setCondition(config, ConditionTypeReady, metav1.ConditionTrue, ReasonReconcileSuccess, "Successfully reconciled RBAC")
	r.setCondition(config, ConditionTypeProgressing, metav1.ConditionFalse, ReasonReconcileSuccess, "Reconciliation completed")
	r.setCondition(config, ConditionTypeDegraded, metav1.ConditionFalse, ReasonReconcileSuccess, "No issues detected")

	return r.updateStatus(ctx, config, log)
}

// handleDeletion handles the deletion of a NamespaceRBACConfig
func (r *NamespaceRBACConfigReconciler) handleDeletion(ctx context.Context, config *rbacoperatorv1.NamespaceRBACConfig, log logr.Logger) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(config, FinalizerName) {
		log.Info("Cleaning up RBAC resources for deleted NamespaceRBACConfig")

		// Clean up RBAC resources
		if err := r.cleanupRBAC(ctx, config, log); err != nil {
			log.Error(err, "Failed to cleanup RBAC resources")
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(config, FinalizerName)
		if err := r.Update(ctx, config); err != nil {
			log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// validateConfig validates the NamespaceRBACConfig
func (r *NamespaceRBACConfigReconciler) validateConfig(config *rbacoperatorv1.NamespaceRBACConfig) error {
	// Validate namespace selector
	if config.Spec.NamespaceSelector.NameRegex != nil {
		if _, err := regexp.Compile(*config.Spec.NamespaceSelector.NameRegex); err != nil {
			return fmt.Errorf("invalid nameRegex: %w", err)
		}
	}

	// Validate RBAC templates
	// TODO: Add more comprehensive validation
	if len(config.Spec.RBACTemplates.Roles) == 0 &&
		len(config.Spec.RBACTemplates.ClusterRoles) == 0 &&
		len(config.Spec.RBACTemplates.RoleBindings) == 0 &&
		len(config.Spec.RBACTemplates.ClusterRoleBindings) == 0 {
		return fmt.Errorf("at least one RBAC template must be specified")
	}

	return nil
}

// reconcileRBAC reconciles RBAC for all matching namespaces
func (r *NamespaceRBACConfigReconciler) reconcileRBAC(ctx context.Context, config *rbacoperatorv1.NamespaceRBACConfig, log logr.Logger) ([]string, error) {
	// List all namespaces
	namespaceList := &corev1.NamespaceList{}
	if err := r.List(ctx, namespaceList); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	appliedNamespaces := make([]string, 0)

	// Process each namespace
	for _, ns := range namespaceList.Items {
		// Check if namespace matches selector
		matches, err := utils.NamespaceMatches(&ns, config.Spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "Failed to check namespace match", "namespace", ns.Name)
			continue
		}

		if matches {
			log.Info("Applying RBAC to namespace", "namespace", ns.Name)
			if err := r.rbacManager.ApplyRBACForNamespace(ctx, &ns, config); err != nil {
				return nil, fmt.Errorf("failed to apply RBAC for namespace %s: %w", ns.Name, err)
			}
			appliedNamespaces = append(appliedNamespaces, ns.Name)
		}
	}

	log.Info("Successfully reconciled RBAC", "appliedNamespaces", appliedNamespaces)
	return appliedNamespaces, nil
}

// cleanupRBAC cleans up RBAC resources created by this config
func (r *NamespaceRBACConfigReconciler) cleanupRBAC(ctx context.Context, config *rbacoperatorv1.NamespaceRBACConfig, log logr.Logger) error {
	// For each namespace that was managed by this config
	for _, namespaceName := range config.Status.AppliedNamespaces {
		log.Info("Cleaning up RBAC for namespace", "namespace", namespaceName)
		if err := r.rbacManager.CleanupRBACForNamespace(ctx, namespaceName, config); err != nil {
			log.Error(err, "Failed to cleanup RBAC for namespace", "namespace", namespaceName)
			// Continue with other namespaces even if one fails
		}
	}

	return nil
}

// setCondition sets a condition on the NamespaceRBACConfig status
func (r *NamespaceRBACConfigReconciler) setCondition(config *rbacoperatorv1.NamespaceRBACConfig, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             reason,
		Message:            message,
	}

	// Find existing condition
	for i, existing := range config.Status.Conditions {
		if existing.Type == conditionType {
			// Update existing condition
			if existing.Status != status {
				condition.LastTransitionTime = metav1.NewTime(time.Now())
			} else {
				condition.LastTransitionTime = existing.LastTransitionTime
			}
			config.Status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	config.Status.Conditions = append(config.Status.Conditions, condition)
}

// updateStatus updates the status of the NamespaceRBACConfig
func (r *NamespaceRBACConfigReconciler) updateStatus(ctx context.Context, config *rbacoperatorv1.NamespaceRBACConfig, log logr.Logger) (ctrl.Result, error) {
	if err := r.Status().Update(ctx, config); err != nil {
		if errors.IsNotFound(err) {
			log.Info("NamespaceRBACConfig was deleted during reconciliation, skipping status update")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to update NamespaceRBACConfig status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceRBACConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacoperatorv1.NamespaceRBACConfig{}).
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToConfigs),
		).
		Complete(r)
}

// mapNamespaceToConfigs maps namespace events to NamespaceRBACConfig reconcile requests
func (r *NamespaceRBACConfigReconciler) mapNamespaceToConfigs(ctx context.Context, obj client.Object) []reconcile.Request {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	log := r.Log.WithValues("namespace", namespace.Name)

	// List all NamespaceRBACConfigs
	configList := &rbacoperatorv1.NamespaceRBACConfigList{}
	if err := r.List(ctx, configList); err != nil {
		log.Error(err, "Failed to list NamespaceRBACConfigs")
		return nil
	}

	requests := make([]reconcile.Request, 0)

	// Check which configs should be reconciled for this namespace
	for _, config := range configList.Items {
		matches, err := utils.NamespaceMatches(namespace, config.Spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "Failed to check namespace match", "config", config.Name)
			continue
		}

		if matches {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      config.Name,
					Namespace: config.Namespace,
				},
			})
		}
	}

	return requests
}
