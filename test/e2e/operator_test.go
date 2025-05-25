package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
)

const (
	testTimeout  = 5 * time.Minute
	pollInterval = 10 * time.Second
)

type E2ETestSuite struct {
	client.Client
	clientset *kubernetes.Clientset
	ctx       context.Context
}

func TestMain(m *testing.M) {
	// Run tests
	m.Run()
}

func setupTestSuite(t *testing.T) *E2ETestSuite {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		t.Fatalf("Failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create clientset: %v", err)
	}

	scheme := runtime.NewScheme()
	rbacoperatorv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	rbacv1.AddToScheme(scheme)

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("Failed to create controller-runtime client: %v", err)
	}

	return &E2ETestSuite{
		Client:    k8sClient,
		clientset: clientset,
		ctx:       context.Background(),
	}
}

func TestOperatorBasicFlow(t *testing.T) {
	suite := setupTestSuite(t)

	// Test namespace names
	testNS1 := "e2e-dev-test-app"
	testNS2 := "e2e-prod-app"
	configName := "e2e-test-config"

	// Cleanup function
	defer func() {
		suite.cleanup(t, testNS1, testNS2, configName)
	}()

	// Step 1: Create NamespaceRBACConfig
	t.Log("Creating NamespaceRBACConfig")
	config := suite.createTestConfig(t, configName)

	// Step 2: Create matching namespace
	t.Log("Creating matching namespace")
	suite.createNamespace(t, testNS1, map[string]string{
		"team":                     "platform",
		"rbac.operator.io/managed": "true",
	})

	// Step 3: Verify RBAC resources created
	t.Log("Verifying RBAC resources created")
	suite.verifyRBACCreated(t, testNS1, config)

	// Step 4: Create non-matching namespace
	t.Log("Creating non-matching namespace")
	suite.createNamespace(t, testNS2, map[string]string{
		"team": "other",
	})

	// Step 5: Verify no RBAC resources created for non-matching namespace
	t.Log("Verifying no RBAC for non-matching namespace")
	suite.verifyNoRBAC(t, testNS2)

	// Step 6: Delete matching namespace
	t.Log("Deleting matching namespace")
	suite.deleteNamespace(t, testNS1)

	// Step 7: Verify cleanup
	t.Log("Verifying resource cleanup")
	suite.verifyCleanup(t, testNS1)
}

func (s *E2ETestSuite) createTestConfig(t *testing.T, name string) *rbacoperatorv1.NamespaceRBACConfig {
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^e2e-dev-.*"}[0],
				Annotations: map[string]string{
					"team":                     "platform",
					"rbac.operator.io/managed": "true",
				},
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: "e2e-developer-{{.Namespace.Name}}",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods", "services"},
								Verbs:     []string{"get", "list", "create"},
							},
						},
						Labels: map[string]string{
							"e2e-test": "true",
						},
					},
				},
				RoleBindings: []rbacoperatorv1.RoleBindingTemplate{
					{
						Name: "e2e-developers-{{.Namespace.Name}}",
						RoleRef: rbacv1.RoleRef{
							Kind: "Role",
							Name: "e2e-developer-{{.Namespace.Name}}",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:     "Group",
								Name:     "developers",
								APIGroup: "rbac.authorization.k8s.io",
							},
						},
						Labels: map[string]string{
							"e2e-test": "true",
						},
					},
				},
			},
		},
	}

	err := s.Create(s.ctx, config)
	if err != nil {
		t.Fatalf("Failed to create NamespaceRBACConfig: %v", err)
	}

	return config
}

func (s *E2ETestSuite) createNamespace(t *testing.T, name string, annotations map[string]string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}

	err := s.Create(s.ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace %s: %v", name, err)
	}
}

func (s *E2ETestSuite) verifyRBACCreated(t *testing.T, namespace string, config *rbacoperatorv1.NamespaceRBACConfig) {
	// Wait for and verify Role
	roleName := fmt.Sprintf("e2e-developer-%s", namespace)
	err := wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		role := &rbacv1.Role{}
		err := s.Get(s.ctx, client.ObjectKey{
			Name:      roleName,
			Namespace: namespace,
		}, role)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Verify role has expected label
		if role.Labels["e2e-test"] != "true" {
			t.Errorf("Role missing expected label")
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("Role %s not created in namespace %s: %v", roleName, namespace, err)
	}

	// Wait for and verify RoleBinding
	bindingName := fmt.Sprintf("e2e-developers-%s", namespace)
	err = wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		binding := &rbacv1.RoleBinding{}
		err := s.Get(s.ctx, client.ObjectKey{
			Name:      bindingName,
			Namespace: namespace,
		}, binding)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Verify binding references correct role
		if binding.RoleRef.Name != roleName {
			t.Errorf("RoleBinding references wrong role: got %s, want %s", binding.RoleRef.Name, roleName)
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("RoleBinding %s not created in namespace %s: %v", bindingName, namespace, err)
	}
}

func (s *E2ETestSuite) verifyNoRBAC(t *testing.T, namespace string) {
	// Check that no RBAC resources exist with e2e-test label
	roleList := &rbacv1.RoleList{}
	err := s.List(s.ctx, roleList, client.InNamespace(namespace), client.MatchingLabels{"e2e-test": "true"})
	if err != nil {
		t.Fatalf("Failed to list roles: %v", err)
	}

	if len(roleList.Items) > 0 {
		t.Errorf("Unexpected roles found in namespace %s: %d", namespace, len(roleList.Items))
	}

	bindingList := &rbacv1.RoleBindingList{}
	err = s.List(s.ctx, bindingList, client.InNamespace(namespace), client.MatchingLabels{"e2e-test": "true"})
	if err != nil {
		t.Fatalf("Failed to list rolebindings: %v", err)
	}

	if len(bindingList.Items) > 0 {
		t.Errorf("Unexpected rolebindings found in namespace %s: %d", namespace, len(bindingList.Items))
	}
}

func (s *E2ETestSuite) deleteNamespace(t *testing.T, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := s.Delete(s.ctx, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("Failed to delete namespace %s: %v", name, err)
	}

	// Wait for namespace to be deleted
	err = wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		err := s.Get(s.ctx, client.ObjectKey{Name: name}, ns)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("Namespace %s not deleted: %v", name, err)
	}
}

func (s *E2ETestSuite) verifyCleanup(t *testing.T, namespace string) {
	// Verify namespace-scoped resources are cleaned up automatically
	// (they should be deleted with the namespace)
	roleName := fmt.Sprintf("e2e-developer-%s", namespace)
	role := &rbacv1.Role{}
	err := s.Get(s.ctx, client.ObjectKey{
		Name:      roleName,
		Namespace: namespace,
	}, role)

	if !apierrors.IsNotFound(err) {
		t.Errorf("Role %s still exists after namespace deletion", roleName)
	}
}

func (s *E2ETestSuite) cleanup(t *testing.T, testNS1, testNS2, configName string) {
	// Delete namespaces
	for _, ns := range []string{testNS1, testNS2} {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		s.Delete(s.ctx, namespace)
	}

	// Delete config
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: configName},
	}
	s.Delete(s.ctx, config)

	// Clean up any remaining cluster-scoped resources with e2e-test label
	clusterRoleList := &rbacv1.ClusterRoleList{}
	s.List(s.ctx, clusterRoleList, client.MatchingLabels{"e2e-test": "true"})
	for _, cr := range clusterRoleList.Items {
		s.Delete(s.ctx, &cr)
	}

	clusterBindingList := &rbacv1.ClusterRoleBindingList{}
	s.List(s.ctx, clusterBindingList, client.MatchingLabels{"e2e-test": "true"})
	for _, crb := range clusterBindingList.Items {
		s.Delete(s.ctx, &crb)
	}
}
