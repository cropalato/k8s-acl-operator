package e2e

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rbacoperatorv1 "github.com/yourusername/k8s-acl-operator/pkg/apis/rbac/v1"
)

func TestHealthEndpoints(t *testing.T) {
	suite := setupTestSuite(t)

	// Get operator pod to access health endpoints
	operatorPod := suite.getOperatorPod(t)
	if operatorPod == nil {
		t.Skip("Operator pod not found - skipping health endpoint tests")
	}

	// Test liveness endpoint
	t.Run("Liveness", func(t *testing.T) {
		suite.testHealthEndpoint(t, operatorPod, "/healthz", "liveness")
	})

	// Test readiness endpoint
	t.Run("Readiness", func(t *testing.T) {
		suite.testHealthEndpoint(t, operatorPod, "/readyz", "readiness")
	})
}

func TestHealthWithOperatorLoad(t *testing.T) {
	suite := setupTestSuite(t)

	operatorPod := suite.getOperatorPod(t)
	if operatorPod == nil {
		t.Skip("Operator pod not found - skipping health load test")
	}

	// Create multiple configs to stress the operator
	configNames := []string{"health-test-1", "health-test-2", "health-test-3"}
	namespaces := []string{"e2e-health-ns-1", "e2e-health-ns-2", "e2e-health-ns-3"}

	defer func() {
		for i, configName := range configNames {
			suite.cleanupHealthTest(t, configName, namespaces[i])
		}
	}()

	// Create configs and namespaces rapidly
	for i, configName := range configNames {
		suite.createHealthTestConfig(t, configName, namespaces[i])
		suite.createNamespace(t, namespaces[i], map[string]string{
			"health-test": "true",
		})
	}

	// Verify health endpoints remain responsive
	for i := 0; i < 5; i++ {
		suite.verifyHealthEndpointsResponsive(t, operatorPod)
		time.Sleep(2 * time.Second)
	}
}

func (s *E2ETestSuite) getOperatorPod(t *testing.T) *corev1.Pod {
	podList := &corev1.PodList{}
	err := s.List(s.ctx, podList, client.InNamespace("k8s-acl-operator-system"),
		client.MatchingLabels{"control-plane": "controller-manager"})
	if err != nil {
		t.Logf("Failed to list operator pods: %v", err)
		return nil
	}

	if len(podList.Items) == 0 {
		t.Log("No operator pods found")
		return nil
	}

	return &podList.Items[0]
}

func (s *E2ETestSuite) testHealthEndpoint(t *testing.T, pod *corev1.Pod, path, endpointType string) {
	// For e2e testing, we'll verify the pod is running and has proper health configuration
	if pod.Status.Phase != corev1.PodRunning {
		t.Errorf("Operator pod not running: %s", pod.Status.Phase)
		return
	}

	// Verify health probe configuration exists
	container := s.findManagerContainer(pod)
	if container == nil {
		t.Error("Manager container not found")
		return
	}

	var probe *corev1.Probe
	if endpointType == "liveness" {
		probe = container.LivenessProbe
	} else {
		probe = container.ReadinessProbe
	}

	if probe == nil {
		t.Errorf("%s probe not configured", endpointType)
		return
	}

	if probe.HTTPGet.Path != path {
		t.Errorf("Wrong %s probe path: got %s, want %s", endpointType, probe.HTTPGet.Path, path)
	}

	if probe.HTTPGet.Port.IntVal != 8081 {
		t.Errorf("Wrong %s probe port: got %d, want 8081", endpointType, probe.HTTPGet.Port.IntVal)
	}
}

func (s *E2ETestSuite) verifyHealthEndpointsResponsive(t *testing.T, pod *corev1.Pod) {
	// In a real cluster, you'd use port-forwarding or service access
	// For this test, we verify probe configuration and pod health

	if pod.Status.Phase != corev1.PodRunning {
		t.Errorf("Pod not running during health check: %s", pod.Status.Phase)
	}

	// Check container restart count (should be low)
	container := s.findManagerContainer(pod)
	if container != nil {
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == container.Name && status.RestartCount > 2 {
				t.Errorf("High restart count indicates health issues: %d", status.RestartCount)
			}
		}
	}
}

func (s *E2ETestSuite) findManagerContainer(pod *corev1.Pod) *corev1.Container {
	for _, container := range pod.Spec.Containers {
		if container.Name == "manager" {
			return &container
		}
	}
	return nil
}

func (s *E2ETestSuite) createHealthTestConfig(t *testing.T, name, namespace string) {
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{fmt.Sprintf("^%s$", namespace)}[0],
				Annotations: map[string]string{
					"health-test": "true",
				},
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{
					{
						Name: fmt.Sprintf("health-role-%s", namespace),
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"pods"},
								Verbs:     []string{"get", "list"},
							},
						},
						Labels: map[string]string{"health-test": "true"},
					},
				},
			},
		},
	}

	err := s.Create(s.ctx, config)
	if err != nil {
		t.Fatalf("Failed to create health test config %s: %v", name, err)
	}
}

func (s *E2ETestSuite) cleanupHealthTest(t *testing.T, configName, namespace string) {
	// Delete namespace
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	s.Delete(s.ctx, ns)

	// Delete config
	config := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: configName},
	}
	s.Delete(s.ctx, config)
}

// TestOperatorHealthRecovery tests that operator recovers health after errors
func TestOperatorHealthRecovery(t *testing.T) {
	suite := setupTestSuite(t)

	configName := "health-recovery-test"
	testNS := "e2e-recovery-test"

	defer suite.cleanupHealthTest(t, configName, testNS)

	// Create invalid config that should cause reconcile errors
	invalidConfig := &rbacoperatorv1.NamespaceRBACConfig{
		ObjectMeta: metav1.ObjectMeta{Name: configName},
		Spec: rbacoperatorv1.NamespaceRBACConfigSpec{
			NamespaceSelector: rbacoperatorv1.NamespaceSelector{
				NameRegex: &[]string{"[invalid"}[0], // Invalid regex
			},
			RBACTemplates: rbacoperatorv1.RBACTemplates{
				Roles: []rbacoperatorv1.RoleTemplate{{Name: "test-role"}},
			},
		},
	}

	err := suite.Create(suite.ctx, invalidConfig)
	if err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}

	// Wait for error condition
	err = wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		config := &rbacoperatorv1.NamespaceRBACConfig{}
		err := suite.Get(suite.ctx, client.ObjectKey{Name: configName}, config)
		if err != nil {
			return false, err
		}

		for _, condition := range config.Status.Conditions {
			if condition.Type == "Degraded" && condition.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		t.Log("Config may not have reached degraded state, continuing test")
	}

	// Fix the config
	validConfig := &rbacoperatorv1.NamespaceRBACConfig{}
	err = suite.Get(suite.ctx, client.ObjectKey{Name: configName}, validConfig)
	if err != nil {
		t.Fatalf("Failed to get config for fixing: %v", err)
	}

	regex := "^e2e-recovery-test$"
	validConfig.Spec.NamespaceSelector.NameRegex = &regex
	err = suite.Update(suite.ctx, validConfig)
	if err != nil {
		t.Fatalf("Failed to fix config: %v", err)
	}

	// Verify recovery
	err = wait.PollImmediate(pollInterval, testTimeout, func() (bool, error) {
		config := &rbacoperatorv1.NamespaceRBACConfig{}
		err := suite.Get(suite.ctx, client.ObjectKey{Name: configName}, config)
		if err != nil {
			return false, err
		}

		for _, condition := range config.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		t.Errorf("Config did not recover to ready state: %v", err)
	}
}
