package namespacerbacconfig

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
	"github.com/yourusername/k8s-acl-operator/pkg/health"
	"github.com/yourusername/k8s-acl-operator/pkg/metrics"
)

func TestUpdateStatusNotFoundHandling(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	// Create fake client without the config object
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceRBACConfigReconciler(fakeClient, scheme, log.Log, healthChecker)

	// Create a config that doesn't exist in the client
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-existent-config",
		},
		Status: rbacoperatorv1.NamespaceRBACConfigStatus{
			AppliedNamespaces: []string{"test-ns"},
		},
	}

	// updateStatus should handle NotFound error gracefully
	result, err := reconciler.updateStatus(context.Background(), config, log.Log)
	if err != nil {
		t.Errorf("updateStatus should handle NotFound gracefully, got error: %v", err)
	}

	// Should return empty result without requeue
	if result.Requeue || result.RequeueAfter > 0 {
		t.Errorf("updateStatus should not requeue on NotFound error")
	}
}

func TestReconcileCreateConfig(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^test-.*"}[0],
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: "test-role",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get"},
							},
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceRBACConfigReconciler(fakeClient, scheme, log.Log, healthChecker)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-config",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Log("Requeue expected for finalizer addition")
	}

	// Verify finalizer was added
	updated := &rbacoperatorv1.NamespaceRBACConfig{}
	err = fakeClient.Get(context.Background(), req.NamespacedName, updated)
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	found := false
	for _, finalizer := range updated.Finalizers {
		if finalizer == FinalizerName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Finalizer not added")
	}
}

func TestValidateConfig(t *testing.T) {
	reconciler := &NamespaceRBACConfigReconciler{}

	tests := []struct {
		name    string
		config  *rbacoperatorv1.NamespaceRBACConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &rbacoperatorv1.NamespaceRBACConfig{
				Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
					NamespaceSelector: rbacoperatorv1.NamespaceSelector{
						NameRegex: &[]string{"^test-.*"}[0],
					},
					RBACTemplates: rbacoperatorv1.RBACTemplates{
						Roles: []rbacoperatorv1.RoleTemplate{
							{Name: "test", Rules: []rbacv1.PolicyRule{{Verbs: []string{"get"}}}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex",
			config: &rbacoperatorv1.NamespaceRBACConfig{
				Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
					NamespaceSelector: rbacoperatorv1.NamespaceSelector{
						NameRegex: &[]string{"[invalid"}[0],
					},
					RBACTemplates: rbacoperatorv1.RBACTemplates{
						Roles: []rbacoperatorv1.RoleTemplate{
							{Name: "test", Rules: []rbacv1.PolicyRule{{Verbs: []string{"get"}}}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no templates",
			config: &rbacoperatorv1.NamespaceRBACConfig{
				Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
					NamespaceSelector: rbacoperatorv1.NamespaceSelector{
						NameRegex: &[]string{"^test-.*"}[0],
					},
					RBACTemplates: rbacoperatorv1.RBACTemplates{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconciler.validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMapNamespaceToConfigs(t *testing.T) {
	// Reset metrics before test
	metrics.ResetMetrics()

	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^test-.*"}[0],
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{{Name: "test"}},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceRBACConfigReconciler(fakeClient, scheme, log.Log, healthChecker)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
	}

	requests := reconciler.mapNamespaceToConfigs(context.Background(), ns)

	if len(requests) != 1 {
		t.Errorf("Expected 1 request, got %d", len(requests))
	}

	if requests[0].Name != "test-config" {
		t.Errorf("Expected config test-config, got %s", requests[0].Name)
	}
}
