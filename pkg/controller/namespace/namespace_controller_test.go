package namespace

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
)

func TestNamespaceReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacv1.AddToScheme(scheme)

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^test-.*"}[0],
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: "test-role",
						Rules: []rbacv1.PolicyRule{
							{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
						},
					},
				},
			},
		},
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(config, namespace).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceReconciler(fakeClient, scheme, log.Log, healthChecker)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-namespace"},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if result.Requeue {
		t.Error("Should not requeue")
	}

	// Verify health was recorded
	if !healthChecker.IsHealthy() {
		t.Error("Health should be recorded after successful reconcile")
	}
}

func TestNamespaceReconcilerDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{".*"}[0],
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{{Name: "test-role"}},
			},
		},
	}

	// No namespace exists (simulating deletion)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceReconciler(fakeClient, scheme, log.Log, healthChecker)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleted-namespace"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Should handle deletion gracefully and record health
	if !healthChecker.IsHealthy() {
		t.Error("Health should be recorded after handling deletion")
	}
}

func TestNamespaceReconcilerError(t *testing.T) {
	scheme := runtime.NewScheme()
	// Don't add rbacoperatorv1 to scheme to trigger error

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace).Build()
	healthChecker := health.NewChecker(log.Log)
	reconciler := NewNamespaceReconciler(fakeClient, scheme, log.Log, healthChecker)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-namespace"},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	if err == nil {
		t.Error("Expected error due to scheme mismatch")
	}

	// Health should be marked unhealthy on error
	if healthChecker.IsHealthy() {
		t.Error("Health should be unhealthy after error")
	}
}
