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

// Package rbac manages RBAC resource creation and updates.
// It handles template processing, conflict resolution, and merge strategies.
// The manager processes RBAC templates and applies them to namespaces,
// handling conflicts through configurable merge strategies.
package rbac

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rbacoperatorv1 "github.com/cropalato/k8s-acl-operator/pkg/apis/rbac/v1"
	"github.com/cropalato/k8s-acl-operator/pkg/metrics"
	"github.com/cropalato/k8s-acl-operator/pkg/template"
)

const (
	// OwnerLabel marks resources as owned by the operator for tracking and cleanup
	OwnerLabel = "rbac.operator.io/owned-by"
	// ConfigLabel references the creating NamespaceRBACConfig for resource relationships
	ConfigLabel = "rbac.operator.io/config"
	// NamespaceLabel references the target namespace for cluster-scoped resources
	NamespaceLabel = "rbac.operator.io/namespace"
)

// Manager handles RBAC resource creation and management.
// It processes templates from NamespaceRBACConfig resources and applies them
// to namespaces, handling conflicts through configurable merge strategies.
// The manager ensures proper labeling and ownership of created resources.
type Manager struct {
	client.Client                   // Kubernetes API client for CRUD operations
	templateEngine *template.Engine // Template processor for variable substitution
}

// NewManager creates a new RBAC manager
func NewManager(client client.Client) *Manager {
	return &Manager{
		Client:         client,
		templateEngine: template.NewEngine(),
	}
}

// ApplyRBACForNamespace applies all RBAC templates from a config to a specific namespace.
// It processes roles, cluster roles, role bindings, and cluster role bindings in sequence.
// Template variables are substituted with actual namespace metadata and config values.
// Returns error if any resource creation/update fails.
func (m *Manager) ApplyRBACForNamespace(ctx context.Context, ns *corev1.Namespace, config *rbacoperatorv1.NamespaceRBACConfig) error {
	templateCtx := m.templateEngine.BuildContext(ns, config)

	// Apply Roles
	for _, roleTemplate := range config.Spec.RBACTemplates.Roles {
		if err := m.applyRole(ctx, ns, config, roleTemplate, templateCtx); err != nil {
			return fmt.Errorf("failed to apply role %s: %w", roleTemplate.Name, err)
		}
	}

	// Apply ClusterRoles
	for _, clusterRoleTemplate := range config.Spec.RBACTemplates.ClusterRoles {
		if err := m.applyClusterRole(ctx, ns, config, clusterRoleTemplate, templateCtx); err != nil {
			return fmt.Errorf("failed to apply cluster role %s: %w", clusterRoleTemplate.Name, err)
		}
	}

	// Apply RoleBindings
	for _, roleBindingTemplate := range config.Spec.RBACTemplates.RoleBindings {
		if err := m.applyRoleBinding(ctx, ns, config, roleBindingTemplate, templateCtx); err != nil {
			return fmt.Errorf("failed to apply role binding %s: %w", roleBindingTemplate.Name, err)
		}
	}

	// Apply ClusterRoleBindings
	for _, clusterRoleBindingTemplate := range config.Spec.RBACTemplates.ClusterRoleBindings {
		if err := m.applyClusterRoleBinding(ctx, ns, config, clusterRoleBindingTemplate, templateCtx); err != nil {
			return fmt.Errorf("failed to apply cluster role binding %s: %w", clusterRoleBindingTemplate.Name, err)
		}
	}

	return nil
}

// applyRole creates or updates a Role
func (m *Manager) applyRole(ctx context.Context, ns *corev1.Namespace, config *rbacoperatorv1.NamespaceRBACConfig, template rbacoperatorv1.RoleTemplate, templateCtx *template.TemplateContext) error {
	start := time.Now()
	name, err := m.templateEngine.ProcessTemplate(template.Name, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "role_name", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process role name template: %w", err)
	}

	start = time.Now()
	labels, err := m.templateEngine.ProcessMap(template.Labels, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "role_labels", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process role labels: %w", err)
	}

	start = time.Now()
	annotations, err := m.templateEngine.ProcessMap(template.Annotations, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "role_annotations", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process role annotations: %w", err)
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns.Name,
			Labels:      m.mergeLabels(labels, config, ns.Name),
			Annotations: annotations,
		},
		Rules: template.Rules,
	}

	// Set owner reference to the namespace
	if err := controllerutil.SetControllerReference(ns, role, m.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err = m.createOrUpdateRole(ctx, role, config)
	// Record resource operation
	operation := "create"
	if err == nil {
		// Check if it was create or update by checking if resource already existed
		existing := &rbacv1.Role{}
		if getErr := m.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, existing); getErr == nil {
			operation = "update"
		}
	}
	metrics.RecordResourceOperation(config.Name, "role", operation, err)

	// Update managed resources count
	if err == nil {
		metrics.UpdateManagedResources(config.Name, "role", ns.Name, 1)
	}

	return err
}

// applyClusterRole creates or updates a ClusterRole
func (m *Manager) applyClusterRole(ctx context.Context, ns *corev1.Namespace, config *rbacoperatorv1.NamespaceRBACConfig, template rbacoperatorv1.ClusterRoleTemplate, templateCtx *template.TemplateContext) error {
	start := time.Now()
	name, err := m.templateEngine.ProcessTemplate(template.Name, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "clusterrole_name", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process cluster role name template: %w", err)
	}

	labels, err := m.templateEngine.ProcessMap(template.Labels, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process cluster role labels: %w", err)
	}

	annotations, err := m.templateEngine.ProcessMap(template.Annotations, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process cluster role annotations: %w", err)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      m.mergeLabels(labels, config, ns.Name),
			Annotations: annotations,
		},
		Rules: template.Rules,
	}

	err = m.createOrUpdateClusterRole(ctx, clusterRole, config)
	metrics.RecordResourceOperation(config.Name, "clusterrole", "create", err)
	if err == nil {
		metrics.UpdateManagedResources(config.Name, "clusterrole", "", 1)
	}
	return err
}

// applyRoleBinding creates or updates a RoleBinding
func (m *Manager) applyRoleBinding(ctx context.Context, ns *corev1.Namespace, config *rbacoperatorv1.NamespaceRBACConfig, template rbacoperatorv1.RoleBindingTemplate, templateCtx *template.TemplateContext) error {
	start := time.Now()
	name, err := m.templateEngine.ProcessTemplate(template.Name, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "rolebinding_name", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process role binding name template: %w", err)
	}

	labels, err := m.templateEngine.ProcessMap(template.Labels, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process role binding labels: %w", err)
	}

	annotations, err := m.templateEngine.ProcessMap(template.Annotations, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process role binding annotations: %w", err)
	}

	// Process role reference name
	roleRefName, err := m.templateEngine.ProcessTemplate(template.RoleRef.Name, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process role ref name template: %w", err)
	}

	// Process subjects
	subjects, err := m.processSubjects(template.Subjects, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process subjects: %w", err)
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns.Name,
			Labels:      m.mergeLabels(labels, config, ns.Name),
			Annotations: annotations,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: template.RoleRef.APIGroup,
			Kind:     template.RoleRef.Kind,
			Name:     roleRefName,
		},
		Subjects: subjects,
	}

	// Set owner reference to the namespace
	if err := controllerutil.SetControllerReference(ns, roleBinding, m.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err = m.createOrUpdateRoleBinding(ctx, roleBinding, config)
	metrics.RecordResourceOperation(config.Name, "rolebinding", "create", err)
	if err == nil {
		metrics.UpdateManagedResources(config.Name, "rolebinding", ns.Name, 1)
	}
	return err
}

// applyClusterRoleBinding creates or updates a ClusterRoleBinding
func (m *Manager) applyClusterRoleBinding(ctx context.Context, ns *corev1.Namespace, config *rbacoperatorv1.NamespaceRBACConfig, template rbacoperatorv1.ClusterRoleBindingTemplate, templateCtx *template.TemplateContext) error {
	start := time.Now()
	name, err := m.templateEngine.ProcessTemplate(template.Name, templateCtx)
	metrics.RecordTemplateProcessing(config.Name, "clusterrolebinding_name", time.Since(start), err)
	if err != nil {
		return fmt.Errorf("failed to process cluster role binding name template: %w", err)
	}

	labels, err := m.templateEngine.ProcessMap(template.Labels, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process cluster role binding labels: %w", err)
	}

	annotations, err := m.templateEngine.ProcessMap(template.Annotations, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process cluster role binding annotations: %w", err)
	}

	// Process role reference name
	roleRefName, err := m.templateEngine.ProcessTemplate(template.RoleRef.Name, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process role ref name template: %w", err)
	}

	// Process subjects
	subjects, err := m.processSubjects(template.Subjects, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to process subjects: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      m.mergeLabels(labels, config, ns.Name),
			Annotations: annotations,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: template.RoleRef.APIGroup,
			Kind:     template.RoleRef.Kind,
			Name:     roleRefName,
		},
		Subjects: subjects,
	}

	err = m.createOrUpdateClusterRoleBinding(ctx, clusterRoleBinding, config)
	metrics.RecordResourceOperation(config.Name, "clusterrolebinding", "create", err)
	if err == nil {
		metrics.UpdateManagedResources(config.Name, "clusterrolebinding", "", 1)
	}
	return err
}

// processSubjects processes template variables in subjects
func (m *Manager) processSubjects(subjects []rbacv1.Subject, templateCtx *template.TemplateContext) ([]rbacv1.Subject, error) {
	result := make([]rbacv1.Subject, len(subjects))

	for i, subject := range subjects {
		processedName, err := m.templateEngine.ProcessTemplate(subject.Name, templateCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to process subject name: %w", err)
		}

		result[i] = rbacv1.Subject{
			Kind:     subject.Kind,
			APIGroup: subject.APIGroup,
			Name:     processedName,
		}

		// Process namespace for ServiceAccount subjects
		if subject.Namespace != "" {
			processedNamespace, err := m.templateEngine.ProcessTemplate(subject.Namespace, templateCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to process subject namespace: %w", err)
			}
			result[i].Namespace = processedNamespace
		}
	}

	return result, nil
}

// mergeLabels merges template labels with operator-managed labels
func (m *Manager) mergeLabels(templateLabels map[string]string, config *rbacoperatorv1.NamespaceRBACConfig, targetNamespace string) map[string]string {
	labels := make(map[string]string)

	// Add template labels
	for k, v := range templateLabels {
		labels[k] = v
	}

	// Add operator-managed labels
	labels[OwnerLabel] = "namespace-rbac-operator"
	labels[ConfigLabel] = config.Name
	if targetNamespace != "" {
		labels[NamespaceLabel] = targetNamespace
	}

	return labels
}

// createOrUpdateRole creates or updates a Role based on merge strategy
func (m *Manager) createOrUpdateRole(ctx context.Context, role *rbacv1.Role, config *rbacoperatorv1.NamespaceRBACConfig) error {
	retry := 3
	for i := 0; i < retry; i++ {
		existing := &rbacv1.Role{}
		err := m.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, existing)

		if errors.IsNotFound(err) {
			return m.Create(ctx, role)
		}
		if err != nil {
			return err
		}

		// Handle merge strategy
		mergeStrategy := rbacoperatorv1.MergeStrategyMerge
		if config.Spec.Config != nil && config.Spec.Config.MergeStrategy != nil {
			mergeStrategy = *config.Spec.Config.MergeStrategy
		}

		switch mergeStrategy {
		case rbacoperatorv1.MergeStrategyIgnore:
			metrics.RecordConflictResolution(config.Name, "ignore", "role")
			return nil // Don't update existing resource
		case rbacoperatorv1.MergeStrategyReplace:
			metrics.RecordConflictResolution(config.Name, "replace", "role")
			role.ResourceVersion = existing.ResourceVersion
			err = m.Update(ctx, role)
		case rbacoperatorv1.MergeStrategyMerge:
			metrics.RecordConflictResolution(config.Name, "merge", "role")
			// Merge rules and update
			role.Rules = mergeRules(existing.Rules, role.Rules)
			role.ResourceVersion = existing.ResourceVersion
			err = m.Update(ctx, role)
		default:
			return fmt.Errorf("unknown merge strategy: %s", mergeStrategy)
		}

		// If no conflict, return
		if err == nil || !errors.IsConflict(err) {
			return err
		}

		// Retry on conflict
	}
	return fmt.Errorf("failed to update role after %d retries due to conflicts", retry)
}

// createOrUpdateClusterRole creates or updates a ClusterRole
func (m *Manager) createOrUpdateClusterRole(ctx context.Context, clusterRole *rbacv1.ClusterRole, config *rbacoperatorv1.NamespaceRBACConfig) error {
	existing := &rbacv1.ClusterRole{}
	err := m.Get(ctx, types.NamespacedName{Name: clusterRole.Name}, existing)

	if errors.IsNotFound(err) {
		return m.Create(ctx, clusterRole)
	}
	if err != nil {
		return err
	}

	// Handle merge strategy
	mergeStrategy := rbacoperatorv1.MergeStrategyMerge
	if config.Spec.Config != nil && config.Spec.Config.MergeStrategy != nil {
		mergeStrategy = *config.Spec.Config.MergeStrategy
	}

	switch mergeStrategy {
	case rbacoperatorv1.MergeStrategyIgnore:
		metrics.RecordConflictResolution(config.Name, "ignore", "clusterrole")
		return nil
	case rbacoperatorv1.MergeStrategyReplace:
		metrics.RecordConflictResolution(config.Name, "replace", "clusterrole")
		clusterRole.ResourceVersion = existing.ResourceVersion
		return m.Update(ctx, clusterRole)
	case rbacoperatorv1.MergeStrategyMerge:
		metrics.RecordConflictResolution(config.Name, "merge", "clusterrole")
		clusterRole.Rules = mergeRules(existing.Rules, clusterRole.Rules)
		clusterRole.ResourceVersion = existing.ResourceVersion
		return m.Update(ctx, clusterRole)
	default:
		return fmt.Errorf("unknown merge strategy: %s", mergeStrategy)
	}
}

// createOrUpdateRoleBinding creates or updates a RoleBinding
func (m *Manager) createOrUpdateRoleBinding(ctx context.Context, roleBinding *rbacv1.RoleBinding, config *rbacoperatorv1.NamespaceRBACConfig) error {
	retry := 3
	for i := 0; i < retry; i++ {
		existing := &rbacv1.RoleBinding{}
		err := m.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existing)

		if errors.IsNotFound(err) {
			return m.Create(ctx, roleBinding)
		}
		if err != nil {
			return err
		}

		// Handle merge strategy
		mergeStrategy := rbacoperatorv1.MergeStrategyMerge
		if config.Spec.Config != nil && config.Spec.Config.MergeStrategy != nil {
			mergeStrategy = *config.Spec.Config.MergeStrategy
		}

		switch mergeStrategy {
		case rbacoperatorv1.MergeStrategyIgnore:
			metrics.RecordConflictResolution(config.Name, "ignore", "rolebinding")
			return nil
		case rbacoperatorv1.MergeStrategyReplace:
			metrics.RecordConflictResolution(config.Name, "replace", "rolebinding")
			roleBinding.ResourceVersion = existing.ResourceVersion
			err = m.Update(ctx, roleBinding)
		case rbacoperatorv1.MergeStrategyMerge:
			metrics.RecordConflictResolution(config.Name, "merge", "rolebinding")
			roleBinding.Subjects = mergeSubjects(existing.Subjects, roleBinding.Subjects)
			roleBinding.ResourceVersion = existing.ResourceVersion
			err = m.Update(ctx, roleBinding)
		default:
			return fmt.Errorf("unknown merge strategy: %s", mergeStrategy)
		}

		if err == nil || !errors.IsConflict(err) {
			return err
		}
	}
	return fmt.Errorf("failed to update rolebinding after %d retries due to conflicts", retry)
}

// createOrUpdateClusterRoleBinding creates or updates a ClusterRoleBinding
func (m *Manager) createOrUpdateClusterRoleBinding(ctx context.Context, clusterRoleBinding *rbacv1.ClusterRoleBinding, config *rbacoperatorv1.NamespaceRBACConfig) error {
	existing := &rbacv1.ClusterRoleBinding{}
	err := m.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.Name}, existing)

	if errors.IsNotFound(err) {
		return m.Create(ctx, clusterRoleBinding)
	}
	if err != nil {
		return err
	}

	// Handle merge strategy
	mergeStrategy := rbacoperatorv1.MergeStrategyMerge
	if config.Spec.Config != nil && config.Spec.Config.MergeStrategy != nil {
		mergeStrategy = *config.Spec.Config.MergeStrategy
	}

	switch mergeStrategy {
	case rbacoperatorv1.MergeStrategyIgnore:
		metrics.RecordConflictResolution(config.Name, "ignore", "clusterrolebinding")
		return nil
	case rbacoperatorv1.MergeStrategyReplace:
		metrics.RecordConflictResolution(config.Name, "replace", "clusterrolebinding")
		clusterRoleBinding.ResourceVersion = existing.ResourceVersion
		return m.Update(ctx, clusterRoleBinding)
	case rbacoperatorv1.MergeStrategyMerge:
		metrics.RecordConflictResolution(config.Name, "merge", "clusterrolebinding")
		clusterRoleBinding.Subjects = mergeSubjects(existing.Subjects, clusterRoleBinding.Subjects)
		clusterRoleBinding.ResourceVersion = existing.ResourceVersion
		return m.Update(ctx, clusterRoleBinding)
	default:
		return fmt.Errorf("unknown merge strategy: %s", mergeStrategy)
	}
}

// mergeRules merges RBAC policy rules
func mergeRules(existing, new []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	// Simple merge - add new rules to existing ones
	// In a production implementation, you might want to deduplicate or merge overlapping rules
	result := make([]rbacv1.PolicyRule, len(existing))
	copy(result, existing)
	result = append(result, new...)
	return result
}

// mergeSubjects merges RBAC subjects
func mergeSubjects(existing, new []rbacv1.Subject) []rbacv1.Subject {
	// Simple merge - add new subjects to existing ones, avoiding duplicates
	subjectMap := make(map[string]rbacv1.Subject)

	// Add existing subjects
	for _, subject := range existing {
		key := fmt.Sprintf("%s/%s/%s/%s", subject.Kind, subject.APIGroup, subject.Name, subject.Namespace)
		subjectMap[key] = subject
	}

	// Add new subjects
	for _, subject := range new {
		key := fmt.Sprintf("%s/%s/%s/%s", subject.Kind, subject.APIGroup, subject.Name, subject.Namespace)
		subjectMap[key] = subject
	}

	// Convert back to slice
	result := make([]rbacv1.Subject, 0, len(subjectMap))
	for _, subject := range subjectMap {
		result = append(result, subject)
	}

	return result
}

// CleanupRBACForNamespace removes RBAC resources for a deleted namespace
func (m *Manager) CleanupRBACForNamespace(ctx context.Context, namespaceName string, config *rbacoperatorv1.NamespaceRBACConfig) error {
	// Cleanup namespace-scoped resources (they should be auto-deleted with the namespace)
	// Focus on cluster-scoped resources that need manual cleanup

	// Cleanup ClusterRoles if no other namespaces reference them
	for _, clusterRoleTemplate := range config.Spec.RBACTemplates.ClusterRoles {
		err := m.cleanupClusterRoleIfOrphaned(ctx, clusterRoleTemplate.Name, namespaceName, config)
		metrics.RecordCleanup("clusterrole", err)
		if err != nil {
			return fmt.Errorf("failed to cleanup cluster role: %w", err)
		}
	}

	// Cleanup ClusterRoleBindings if no other namespaces reference them
	for _, clusterRoleBindingTemplate := range config.Spec.RBACTemplates.ClusterRoleBindings {
		err := m.cleanupClusterRoleBindingIfOrphaned(ctx, clusterRoleBindingTemplate.Name, namespaceName, config)
		metrics.RecordCleanup("clusterrolebinding", err)
		if err != nil {
			return fmt.Errorf("failed to cleanup cluster role binding: %w", err)
		}
	}

	return nil
}

// cleanupClusterRoleIfOrphaned removes a ClusterRole if no namespaces reference it
func (m *Manager) cleanupClusterRoleIfOrphaned(ctx context.Context, nameTemplate, namespaceName string, config *rbacoperatorv1.NamespaceRBACConfig) error {
	// This is a simplified implementation
	// In production, you'd want to check if other namespaces still reference this ClusterRole
	// For now, we'll implement basic cleanup logic

	// Check cleanup configuration
	if config.Spec.Config == nil || config.Spec.Config.Cleanup == nil ||
		config.Spec.Config.Cleanup.DeleteOrphanedClusterResources == nil ||
		!*config.Spec.Config.Cleanup.DeleteOrphanedClusterResources {
		return nil // Cleanup disabled
	}

	// TODO: Implement reference counting logic
	// This would involve checking all namespaces that match the selector
	// and seeing if any still exist and would generate this same ClusterRole name

	return nil
}

// cleanupClusterRoleBindingIfOrphaned removes a ClusterRoleBinding if no namespaces reference it
func (m *Manager) cleanupClusterRoleBindingIfOrphaned(ctx context.Context, nameTemplate, namespaceName string, config *rbacoperatorv1.NamespaceRBACConfig) error {
	// Similar to cleanupClusterRoleIfOrphaned
	if config.Spec.Config == nil || config.Spec.Config.Cleanup == nil ||
		config.Spec.Config.Cleanup.DeleteOrphanedClusterResources == nil ||
		!*config.Spec.Config.Cleanup.DeleteOrphanedClusterResources {
		return nil
	}

	// TODO: Implement reference counting logic

	return nil
}
