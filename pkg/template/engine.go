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

// Package template provides a Go template engine for processing RBAC resource templates.
// It handles variable substitution for namespace metadata, configuration values,
// and custom variables defined in NamespaceRBACConfig resources.
//
// The engine supports template functions for safe access to potentially missing values:
// - getOrDefault: Get map value with fallback
// - hasKey: Check if map contains key
// - default: Return default value for empty/nil values
package template

import (
	"bytes"
	"fmt"
	"text/template"

	rbacv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	corev1 "k8s.io/api/core/v1"
)

// TemplateContext provides variables available to templates
type TemplateContext struct {
	// Namespace provides access to the target namespace
	Namespace NamespaceContext `json:"namespace"`
	// CRD provides access to the NamespaceRBACConfig metadata
	CRD CRDContext `json:"crd"`
	// Config provides access to configuration values
	Config ConfigContext `json:"config"`
	// CustomVars provides access to custom template variables
	CustomVars map[string]string `json:"customVars"`
}

// NamespaceContext provides namespace information to templates
type NamespaceContext struct {
	// Name of the namespace
	Name string `json:"name"`
	// Labels on the namespace
	Labels map[string]string `json:"labels"`
	// Annotations on the namespace
	Annotations map[string]string `json:"annotations"`
}

// CRDContext provides NamespaceRBACConfig information to templates
type CRDContext struct {
	// Name of the NamespaceRBACConfig
	Name string `json:"name"`
	// Namespace of the NamespaceRBACConfig (empty for cluster-scoped)
	Namespace string `json:"namespace"`
}

// ConfigContext provides configuration information to templates
type ConfigContext struct {
	// Naming configuration
	Naming NamingContext `json:"naming"`
}

// NamingContext provides naming configuration to templates
type NamingContext struct {
	// Prefix for generated resource names
	Prefix string `json:"prefix"`
	// Suffix for generated resource names
	Suffix string `json:"suffix"`
	// Separator for name components
	Separator string `json:"separator"`
}

// Engine handles template processing
type Engine struct {
	funcMap template.FuncMap
}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	return &Engine{
		funcMap: template.FuncMap{
			// Helper functions for safe template processing
			"default": func(defaultVal, val interface{}) interface{} {
				if val == nil || val == "" {
					return defaultVal
				}
				return val
			},
			"hasKey": func(m map[string]string, key string) bool {
				if m == nil {
					return false
				}
				_, exists := m[key]
				return exists
			},
			"getOrDefault": func(m map[string]string, key, defaultVal string) string {
				if m == nil {
					return defaultVal
				}
				if val, exists := m[key]; exists {
					return val
				}
				return defaultVal
			},
		},
	}
}

// BuildContext creates a template context from a namespace and config
func (e *Engine) BuildContext(ns *corev1.Namespace, config *rbacv1.NamespaceRBACConfig) *TemplateContext {
	ctx := &TemplateContext{
		Namespace: NamespaceContext{
			Name:        ns.Name,
			Labels:      ns.Labels,
			Annotations: ns.Annotations,
		},
		CRD: CRDContext{
			Name:      config.Name,
			Namespace: config.Namespace,
		},
		Config: ConfigContext{
			Naming: NamingContext{
				Separator: "-", // default
			},
		},
		CustomVars: make(map[string]string),
	}

	// Ensure maps are not nil
	if ctx.Namespace.Labels == nil {
		ctx.Namespace.Labels = make(map[string]string)
	}
	if ctx.Namespace.Annotations == nil {
		ctx.Namespace.Annotations = make(map[string]string)
	}

	// Apply configuration if provided
	if config.Spec.Config != nil {
		if config.Spec.Config.Naming != nil {
			if config.Spec.Config.Naming.Prefix != "" {
				ctx.Config.Naming.Prefix = config.Spec.Config.Naming.Prefix
			}
			if config.Spec.Config.Naming.Suffix != "" {
				ctx.Config.Naming.Suffix = config.Spec.Config.Naming.Suffix
			}
			if config.Spec.Config.Naming.Separator != "" {
				ctx.Config.Naming.Separator = config.Spec.Config.Naming.Separator
			}
		}

		if config.Spec.Config.TemplateVariables != nil {
			ctx.CustomVars = config.Spec.Config.TemplateVariables
		}
	}

	return ctx
}

// ProcessTemplate processes a template string with the given context
func (e *Engine) ProcessTemplate(templateStr string, ctx *TemplateContext) (string, error) {
	tmpl, err := template.New("resource").Funcs(e.funcMap).Option("missingkey=error").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ProcessMap processes a map of template strings
func (e *Engine) ProcessMap(templateMap map[string]string, ctx *TemplateContext) (map[string]string, error) {
	if templateMap == nil {
		return nil, nil
	}

	result := make(map[string]string)
	for key, value := range templateMap {
		processed, err := e.ProcessTemplate(value, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to process template for key %s: %w", key, err)
		}
		result[key] = processed
	}

	return result, nil
}

// ValidateTemplate validates a template string without executing it
func (e *Engine) ValidateTemplate(templateStr string) error {
	_, err := template.New("validation").Funcs(e.funcMap).Parse(templateStr)
	return err
}
