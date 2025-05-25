package template

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
)

func TestTemplateHelperFunctions(t *testing.T) {
	engine := NewEngine()
	ctx := &TemplateContext{
		Namespace: NamespaceContext{
			Labels:      map[string]string{"existing": "value"},
			Annotations: map[string]string{"env": "prod"},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "getOrDefault with existing key",
			template: "{{getOrDefault .Namespace.Labels \"existing\" \"default\"}}",
			expected: "value",
		},
		{
			name:     "getOrDefault with missing key",
			template: "{{getOrDefault .Namespace.Labels \"missing\" \"default\"}}",
			expected: "default",
		},
		{
			name:     "hasKey returns true for existing",
			template: "{{if hasKey .Namespace.Labels \"existing\"}}found{{else}}notfound{{end}}",
			expected: "found",
		},
		{
			name:     "hasKey returns false for missing",
			template: "{{if hasKey .Namespace.Labels \"missing\"}}found{{else}}notfound{{end}}",
			expected: "notfound",
		},
		{
			name:     "default function with non-empty value",
			template: "{{default \"fallback\" .Namespace.Annotations.env}}",
			expected: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ProcessTemplate(tt.template, ctx)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestTemplateMissingKeyError(t *testing.T) {
	engine := NewEngine()
	ctx := &TemplateContext{
		Namespace: NamespaceContext{
			Labels: map[string]string{},
		},
	}

	// Template that tries to access missing key should error with missingkey=error option
	template := "{{.Namespace.Labels.nonexistent}}"
	_, err := engine.ProcessTemplate(template, ctx)
	if err == nil {
		t.Error("Expected error for missing key access, got none")
	}
}

func TestBuildContext(t *testing.T) {
	engine := NewEngine()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"team": "platform",
			},
			Annotations: map[string]string{
				"env": "dev",
			},
		},
	}

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			Config: &rbacoperatorv1.NamespaceRBACConfigConfig{
				Naming: &rbacoperatorv1.NamingConfig{
					Prefix:    "auto-",
					Suffix:    "-v1",
					Separator: "-",
				},
				TemplateVariables: map[string]string{
					"customVar": "customValue",
				},
			},
		},
	}

	ctx := engine.BuildContext(ns, config)

	if ctx.Namespace.Name != "test-namespace" {
		t.Errorf("Expected namespace name test-namespace, got %s", ctx.Namespace.Name)
	}

	if ctx.Namespace.Labels["team"] != "platform" {
		t.Errorf("Expected team label platform, got %s", ctx.Namespace.Labels["team"])
	}

	if ctx.Config.Naming.Prefix != "auto-" {
		t.Errorf("Expected prefix auto-, got %s", ctx.Config.Naming.Prefix)
	}

	if ctx.CustomVars["customVar"] != "customValue" {
		t.Errorf("Expected customVar customValue, got %s", ctx.CustomVars["customVar"])
	}
}

func TestProcessTemplate(t *testing.T) {
	engine := NewEngine()

	ctx := &TemplateContext{
		Namespace: NamespaceContext{
			Name: "test-ns",
		},
		Config: ConfigContext{
			Naming: NamingContext{
				Prefix:    "prefix-",
				Separator: "-",
			},
		},
		CustomVars: map[string]string{
			"env": "production",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "namespace name",
			template: "role-{{.Namespace.Name}}",
			expected: "role-test-ns",
		},
		{
			name:     "config prefix",
			template: "{{.Config.Naming.Prefix}}{{.Namespace.Name}}",
			expected: "prefix-test-ns",
		},
		{
			name:     "custom variable",
			template: "{{.Namespace.Name}}-{{.CustomVars.env}}",
			expected: "test-ns-production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ProcessTemplate(tt.template, ctx)
			if err != nil {
				t.Fatalf("ProcessTemplate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestProcessMap(t *testing.T) {
	engine := NewEngine()

	ctx := &TemplateContext{
		Namespace: NamespaceContext{Name: "test"},
	}

	templateMap := map[string]string{
		"key1": "value-{{.Namespace.Name}}",
		"key2": "static-value",
	}

	result, err := engine.ProcessMap(templateMap, ctx)
	if err != nil {
		t.Fatalf("ProcessMap failed: %v", err)
	}

	expected := map[string]string{
		"key1": "value-test",
		"key2": "static-value",
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Expected %s=%s, got %s", k, v, result[k])
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name:     "valid template",
			template: "{{.Namespace.Name}}",
			wantErr:  false,
		},
		{
			name:     "invalid template",
			template: "{{.Invalid}}",
			wantErr:  false, // Go templates don't validate field existence
		},
		{
			name:     "syntax error",
			template: "{{.Namespace.Name",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateTemplate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
