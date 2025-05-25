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

package namespace

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	"github.com/yourusername/k8s-acl-operator/pkg/health"
	"github.com/yourusername/k8s-acl-operator/pkg/rbac"
	"github.com/yourusername/k8s-acl-operator/pkg/utils"
)

// NamespaceReconciler reconciles namespace events to trigger RBAC management
type NamespaceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	rbacManager   *rbac.Manager
	healthChecker *health.Checker
}

// NewNamespaceReconciler creates a new namespace reconciler
func NewNamespaceReconciler(client client.Client, scheme *runtime.Scheme, log logr.Logger, healthChecker *health.Checker) *NamespaceReconciler {
	return &NamespaceReconciler{
		Client:        client,
		Scheme:        scheme,
		Log:           log,
		rbacManager:   rbac.NewManager(client),
		healthChecker: healthChecker,
	}
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.operator.io,resources=namespacerbacconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles namespace events and applies/removes RBAC as needed
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.Name)

	// Fetch the namespace
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace was deleted, handle cleanup
			log.Info("Namespace deleted, cleaning up RBAC resources")
			return r.handleNamespaceDeletion(ctx, req.Name, log)
		}
		log.Error(err, "Failed to get namespace")
		r.healthChecker.SetHealthy(false)
		return ctrl.Result{}, err
	}

	// Handle namespace creation/update
	return r.handleNamespaceCreateOrUpdate(ctx, namespace, log)
}

// handleNamespaceCreateOrUpdate handles namespace creation or update events
func (r *NamespaceReconciler) handleNamespaceCreateOrUpdate(ctx context.Context, namespace *corev1.Namespace, log logr.Logger) (ctrl.Result, error) {
	log.Info("Processing namespace create/update event")

	// Get all NamespaceRBACConfigs
	configList := &rbacoperatorv1.NamespaceRBACConfigList{}
	if err := r.List(ctx, configList); err != nil {
		log.Error(err, "Failed to list NamespaceRBACConfigs")
		r.healthChecker.SetHealthy(false)
		return ctrl.Result{}, err
	}

	// Apply RBAC for all matching configs
	for _, config := range configList.Items {
		matches, err := utils.NamespaceMatches(namespace, config.Spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "Failed to check namespace match", "config", config.Name)
			continue
		}

		if matches {
			log.Info("Applying RBAC for namespace", "config", config.Name)
			if err := r.rbacManager.ApplyRBACForNamespace(ctx, namespace, &config); err != nil {
				log.Error(err, "Failed to apply RBAC", "config", config.Name)
				// Continue with other configs even if one fails
			}
		} else {
			// If namespace no longer matches, clean up any previously created resources
			log.Info("Namespace no longer matches config, cleaning up", "config", config.Name)
			if err := r.rbacManager.CleanupRBACForNamespace(ctx, namespace.Name, &config); err != nil {
				log.Error(err, "Failed to cleanup RBAC", "config", config.Name)
				// Continue with other configs even if one fails
			}
		}
	}

	r.healthChecker.RecordReconcile()
	return ctrl.Result{}, nil
}

// handleNamespaceDeletion handles namespace deletion events
func (r *NamespaceReconciler) handleNamespaceDeletion(ctx context.Context, namespaceName string, log logr.Logger) (ctrl.Result, error) {
	log.Info("Processing namespace deletion event")

	// Get all NamespaceRBACConfigs
	configList := &rbacoperatorv1.NamespaceRBACConfigList{}
	if err := r.List(ctx, configList); err != nil {
		log.Error(err, "Failed to list NamespaceRBACConfigs")
		r.healthChecker.SetHealthy(false)
		return ctrl.Result{}, err
	}

	// Clean up RBAC resources for all configs
	for _, config := range configList.Items {
		log.Info("Cleaning up RBAC for deleted namespace", "config", config.Name)
		if err := r.rbacManager.CleanupRBACForNamespace(ctx, namespaceName, &config); err != nil {
			log.Error(err, "Failed to cleanup RBAC", "config", config.Name)
			// Continue with other configs even if one fails
		}
	}

	r.healthChecker.RecordReconcile()
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}
