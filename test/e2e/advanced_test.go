package e2e

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
)

func TestNamespaceSelector(t *testing.T) {
	suite := setupTestSuite(t)

	configName := "e2e-selector-test"
	testNS1 := "e2e-staging-app" // should match
	testNS2 := "e2e-dev-system"  // excluded
	testNS3 := "e2e-other-app"   // no match

	defer func() {
		suite.cleanup(t, testNS1, testNS2, configName)
		// Clean up third namespace manually
		ns3 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS3}}
		suite.Delete(suite.ctx, ns3)
	}()

	// Create config with complex selector
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: configName},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^e2e-(dev|staging)-.*"}[0],
				Annotations: map[string]string{
					"team": "platform",
				},
				ExcludeNamespaces: []string{"e2e-dev-system"},
			},
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
						Labels: map[string]string{"e2e-selector-test": "true"},
					},
				},
			},
		},
	}

	err := suite.Create(suite.ctx, config)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Create namespaces
	testCases := []struct {
		name        string
		annotations map[string]string
		shouldMatch bool
	}{
		{testNS1, map[string]string{"team": "platform"}, true},  // matches regex + annotation
		{testNS2, map[string]string{"team": "platform"}, false}, // excluded
		{testNS3, map[string]string{"team": "platform"}, false}, // doesn't match regex
	}

	for _, tc := range testCases {
		suite.createNamespace(t, tc.name, tc.annotations)

		if tc.shouldMatch {
			suite.verifyRoleExists(t, tc.name, "test-role-"+tc.name)
		} else {
			suite.verifyRoleNotExists(t, tc.name, "test-role-"+tc.name)
		}
	}
}

func TestTemplateVariables(t *testing.T) {
	suite := setupTestSuite(t)

	configName := "e2e-template-test"
	testNS := "e2e-dev-template"

	defer suite.cleanup(t, testNS, "", configName)

	// Create config with template variables
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: configName},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"^e2e-dev-template$"}[0],
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: "{{.Config.Naming.Prefix}}role-{{.Namespace.Name}}{{.Config.Naming.Suffix}}",
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get"},
							},
						},
						Labels: map[string]string{
							"e2e-template-test": "true",
							"custom-var":        "{{.CustomVars.testVar}}",
						},
					},
				},
			},
			Config: &rbacoperatorv1.NamespaceRBACConfigConfig{
				Naming: &rbacoperatorv1.NamingConfig{
					Prefix: "custom-",
					Suffix: "-v1",
				},
				TemplateVariables: map[string]string{
					"testVar": "test-value",
				},
			},
		},
	}

	err := suite.Create(suite.ctx, config)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	suite.createNamespace(t, testNS, nil)

	// Verify role with processed template
	expectedRoleName := "custom-role-" + testNS + "-v1"
	err = wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		role := &rbacv1.Role{}
		err := suite.Get(suite.ctx, client.ObjectKey{
			Name:      expectedRoleName,
			Namespace: testNS,
		}, role)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Verify template variables were processed
		if role.Labels["custom-var"] != "test-value" {
			t.Errorf("Template variable not processed correctly: got %s", role.Labels["custom-var"])
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("Role with template variables not created: %v", err)
	}
}

func (s *E2ETestSuite) verifyRoleExists(t *testing.T, namespace, roleName string) {
	err := wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		role := &rbacv1.Role{}
		err := s.Get(s.ctx, client.ObjectKey{
			Name:      roleName,
			Namespace: namespace,
		}, role)
		return err == nil, err
	})
	if err != nil {
		t.Errorf("Expected role %s not found in namespace %s", roleName, namespace)
	}
}

func (s *E2ETestSuite) verifyRoleNotExists(t *testing.T, namespace, roleName string) {
	// Wait a bit to ensure operator had time to process
	time.Sleep(2 * pollInterval)

	role := &rbacv1.Role{}
	err := s.Get(s.ctx, client.ObjectKey{
		Name:      roleName,
		Namespace: namespace,
	}, role)

	if !apierrors.IsNotFound(err) {
		t.Errorf("Unexpected role %s found in namespace %s", roleName, namespace)
	}
}
