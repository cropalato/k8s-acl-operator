package rbac

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	"github.com/yourusername/k8s-acl-operator/pkg/metrics"
)

func TestCreateOrUpdateRoleConflictRetry(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacoperatorv1.AddToScheme(scheme)

	// Create an existing role first
	existingRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-role",
			Namespace:       "test-ns",
			ResourceVersion: "1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingRole).Build()
	manager := NewManager(fakeClient)

	newRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-role",
			Namespace: "test-ns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"create"},
			},
		},
	}

	config := &rbacoperatorv1.NamespaceRBACConfig{
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			Config: &rbacoperatorv1.NamespaceRBACConfigConfig{
				MergeStrategy: func() *rbacoperatorv1.MergeStrategy {
					s := rbacoperatorv1.MergeStrategyMerge
					return &s
				}(),
			},
		},
	}

	// Should succeed despite potential conflicts due to retry logic
	err := manager.createOrUpdateRole(context.Background(), newRole, config)
	if err != nil {
		t.Fatalf("createOrUpdateRole failed: %v", err)
	}

	// Verify the role was updated with merged rules
	updatedRole := &rbacv1.Role{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-role",
		Namespace: "test-ns",
	}, updatedRole)
	if err != nil {
		t.Fatalf("Failed to get updated role: %v", err)
	}

	if len(updatedRole.Rules) != 2 {
		t.Errorf("Expected 2 rules after merge, got %d", len(updatedRole.Rules))
	}
}

func TestCreateOrUpdateRoleBindingConflictRetry(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacoperatorv1.AddToScheme(scheme)

	// Create an existing role binding first
	existingBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-binding",
			Namespace:       "test-ns",
			ResourceVersion: "1",
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "test-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "developers",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingBinding).Build()
	manager := NewManager(fakeClient)

	newBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "test-ns",
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "test-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "admins",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}

	config := &rbacoperatorv1.NamespaceRBACConfig{
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			Config: &rbacoperatorv1.NamespaceRBACConfigConfig{
				MergeStrategy: func() *rbacoperatorv1.MergeStrategy {
					s := rbacoperatorv1.MergeStrategyMerge
					return &s
				}(),
			},
		},
	}

	// Should succeed despite potential conflicts due to retry logic
	err := manager.createOrUpdateRoleBinding(context.Background(), newBinding, config)
	if err != nil {
		t.Fatalf("createOrUpdateRoleBinding failed: %v", err)
	}

	// Verify the binding was updated with merged subjects
	updatedBinding := &rbacv1.RoleBinding{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-binding",
		Namespace: "test-ns",
	}, updatedBinding)
	if err != nil {
		t.Fatalf("Failed to get updated binding: %v", err)
	}

	if len(updatedBinding.Subjects) != 2 {
		t.Errorf("Expected 2 subjects after merge, got %d", len(updatedBinding.Subjects))
	}
}

func TestApplyRBACForNamespace(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacoperatorv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	manager := NewManager(fakeClient)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"team": "platform",
			},
		},
	}

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: "test-role-{{.Namespace.Name}}",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get", "list"},
							},
						},
						Labels: map[string]string{
							"test": "true",
						},
					},
				},
				RoleBindings: []rbacoperatorv1.RoleBindingTemplate{
					{
						Name: "test-binding-{{.Namespace.Name}}",
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "test-role-{{.Namespace.Name}}",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:     "Group",
								Name:     "developers",
								APIGroup: "rbac.authorization.k8s.io",
							},
						},
					},
				},
			},
		},
	}

	err := manager.ApplyRBACForNamespace(context.Background(), ns, config)
	if err != nil {
		t.Fatalf("ApplyRBACForNamespace failed: %v", err)
	}

	// Verify Role was created
	role := &rbacv1.Role{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-role-test-namespace",
		Namespace: "test-namespace",
	}, role)
	if err != nil {
		t.Fatalf("Role not created: %v", err)
	}

	if role.Labels["test"] != "true" {
		t.Errorf("Role missing expected label")
	}

	if role.Labels[OwnerLabel] != "namespace-rbac-operator" {
		t.Errorf("Role missing owner label")
	}

	// Verify RoleBinding was created
	binding := &rbacv1.RoleBinding{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-binding-test-namespace",
		Namespace: "test-namespace",
	}, binding)
	if err != nil {
		t.Fatalf("RoleBinding not created: %v", err)
	}

	if binding.RoleRef.Name != "test-role-test-namespace" {
		t.Errorf("RoleBinding references wrong role: %s", binding.RoleRef.Name)
	}
}

func TestMergeLabels(t *testing.T) {
	manager := &Manager{}

	templateLabels := map[string]string{
		"custom": "value",
		"env":    "test",
	}

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
	}

	result := manager.mergeLabels(templateLabels, config, "test-namespace")

	expected := map[string]string{
		"custom":       "value",
		"env":          "test",
		OwnerLabel:     "namespace-rbac-operator",
		ConfigLabel:    "test-config",
		NamespaceLabel: "test-namespace",
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("Expected %s=%s, got %s", k, v, result[k])
		}
	}
}

func TestMergeRules(t *testing.T) {
	existing := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get"},
		},
	}

	new := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"create"},
		},
	}

	result := mergeRules(existing, new)

	if len(result) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(result))
	}

	if result[0].Resources[0] != "pods" {
		t.Errorf("First rule should be existing rule")
	}

	if result[1].Resources[0] != "deployments" {
		t.Errorf("Second rule should be new rule")
	}
}

func TestMergeSubjects(t *testing.T) {
	existing := []rbacv1.Subject{
		{
			Kind:     "Group",
			Name:     "developers",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	new := []rbacv1.Subject{
		{
			Kind:     "Group",
			Name:     "admins",
			APIGroup: "rbac.authorization.k8s.io",
		},
		{
			Kind:     "Group",
			Name:     "developers", // duplicate
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	result := mergeSubjects(existing, new)

	if len(result) != 2 {
		t.Errorf("Expected 2 subjects (deduplicated), got %d", len(result))
	}

	names := make(map[string]bool)
	for _, subject := range result {
		names[subject.Name] = true
	}

	if !names["developers"] || !names["admins"] {
		t.Errorf("Missing expected subjects")
	}
}
