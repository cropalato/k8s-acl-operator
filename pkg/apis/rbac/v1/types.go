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

// Package v1 contains API type definitions for the rbac.operator.io/v1 API group.
// This package defines the NamespaceRBACConfig CRD and related types for
// automatic RBAC management in Kubernetes namespaces.
package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NamespaceSelector defines multiple criteria for selecting target namespaces.
// All specified criteria must match (AND logic) except exclusions (take precedence).
type NamespaceSelector struct {
	NameRegex         *string           `json:"nameRegex,omitempty"`         // Regex pattern for namespace names
	Annotations       map[string]string `json:"annotations,omitempty"`       // Required annotations (exact match)
	Labels            map[string]string `json:"labels,omitempty"`            // Required labels (exact match)
	IncludeNamespaces []string          `json:"includeNamespaces,omitempty"` // Explicit inclusion list
	ExcludeNamespaces []string          `json:"excludeNamespaces,omitempty"` // Explicit exclusion list (takes precedence)
}

// RoleTemplate defines a template for creating Roles
type RoleTemplate struct {
	Name        string              `json:"name"`
	Rules       []rbacv1.PolicyRule `json:"rules"`
	Labels      map[string]string   `json:"labels,omitempty"`
	Annotations map[string]string   `json:"annotations,omitempty"`
}

// ClusterRoleTemplate defines a template for creating ClusterRoles
type ClusterRoleTemplate struct {
	Name        string              `json:"name"`
	Rules       []rbacv1.PolicyRule `json:"rules"`
	Labels      map[string]string   `json:"labels,omitempty"`
	Annotations map[string]string   `json:"annotations,omitempty"`
}

// RoleBindingTemplate defines a template for creating RoleBindings
type RoleBindingTemplate struct {
	Name        string            `json:"name"`
	RoleRef     rbacv1.RoleRef    `json:"roleRef"`
	Subjects    []rbacv1.Subject  `json:"subjects"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ClusterRoleBindingTemplate defines a template for creating ClusterRoleBindings
type ClusterRoleBindingTemplate struct {
	Name        string            `json:"name"`
	RoleRef     rbacv1.RoleRef    `json:"roleRef"`
	Subjects    []rbacv1.Subject  `json:"subjects"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// RBACTemplates defines templates for RBAC resources
type RBACTemplates struct {
	Roles               []RoleTemplate               `json:"roles,omitempty"`
	ClusterRoles        []ClusterRoleTemplate        `json:"clusterRoles,omitempty"`
	RoleBindings        []RoleBindingTemplate        `json:"roleBindings,omitempty"`
	ClusterRoleBindings []ClusterRoleBindingTemplate `json:"clusterRoleBindings,omitempty"`
}

// NamingConfig defines naming patterns for generated resources
type NamingConfig struct {
	Prefix    string `json:"prefix,omitempty"`
	Suffix    string `json:"suffix,omitempty"`
	Separator string `json:"separator,omitempty"`
}

// CleanupConfig defines cleanup behavior
type CleanupConfig struct {
	DeleteOrphanedClusterResources *bool  `json:"deleteOrphanedClusterResources,omitempty"`
	GracePeriodSeconds             *int32 `json:"gracePeriodSeconds,omitempty"`
}

// MergeStrategy defines how to handle conflicts when multiple configs
// create resources with the same name.
type MergeStrategy string

const (
	// MergeStrategyMerge combines rules/subjects from all sources
	MergeStrategyMerge MergeStrategy = "merge"
	// MergeStrategyReplace makes the last config win, replacing existing resources
	MergeStrategyReplace MergeStrategy = "replace"
	// MergeStrategyIgnore skips creation if resource already exists
	MergeStrategyIgnore MergeStrategy = "ignore"
)

// NamespaceRBACConfigConfig defines additional configuration options
type NamespaceRBACConfigConfig struct {
	Naming            *NamingConfig     `json:"naming,omitempty"`
	MergeStrategy     *MergeStrategy    `json:"mergeStrategy,omitempty"`
	TemplateVariables map[string]string `json:"templateVariables,omitempty"`
	Cleanup           *CleanupConfig    `json:"cleanup,omitempty"`
}

// NamespaceRBACConfigSpec defines the desired state of NamespaceRBACConfig
type NamespaceRBACConfigSpec struct {
	NamespaceSelector NamespaceSelector          `json:"namespaceSelector"`
	RBACTemplates     RBACTemplates              `json:"rbacTemplates"`
	Config            *NamespaceRBACConfigConfig `json:"config,omitempty"`
}

// ResourceReference tracks a created resource
type ResourceReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// CreatedResources tracks all resources created by this config
type CreatedResources struct {
	Roles               []ResourceReference `json:"roles,omitempty"`
	ClusterRoles        []string            `json:"clusterRoles,omitempty"`
	RoleBindings        []ResourceReference `json:"roleBindings,omitempty"`
	ClusterRoleBindings []string            `json:"clusterRoleBindings,omitempty"`
}

// NamespaceRBACConfigStatus defines the observed state of NamespaceRBACConfig
type NamespaceRBACConfigStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	AppliedNamespaces  []string           `json:"appliedNamespaces,omitempty"`
	CreatedResources   *CreatedResources  `json:"createdResources,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// NamespaceRBACConfig defines automatic RBAC management for namespaces.
// When a namespace matches the selector, the operator creates RBAC resources
// based on the provided templates with variable substitution.
type NamespaceRBACConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceRBACConfigSpec   `json:"spec,omitempty"`
	Status NamespaceRBACConfigStatus `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (in *NamespaceRBACConfig) DeepCopyObject() runtime.Object {
	return &NamespaceRBACConfig{
		TypeMeta:   in.TypeMeta,
		ObjectMeta: *in.ObjectMeta.DeepCopy(),
		Spec:       in.Spec,
		Status:     in.Status,
	}
}

// NamespaceRBACConfigList contains a list of NamespaceRBACConfig
type NamespaceRBACConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceRBACConfig `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (in *NamespaceRBACConfigList) DeepCopyObject() runtime.Object {
	out := &NamespaceRBACConfigList{
		TypeMeta: in.TypeMeta,
		ListMeta: *in.ListMeta.DeepCopy(),
	}
	if in.Items != nil {
		out.Items = make([]NamespaceRBACConfig, len(in.Items))
		for i := range in.Items {
			out.Items[i] = *in.Items[i].DeepCopyObject().(*NamespaceRBACConfig)
		}
	}
	return out
}
