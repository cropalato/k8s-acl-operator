# E2E Tests

End-to-end tests for the k8s-acl-operator.

## Prerequisites

- Kubernetes cluster access
- kubectl configured
- Go 1.21+

## Running Tests

### Full E2E Suite
```bash
make test-e2e
```

### Local Development
```bash
# Run against local operator instance
make test-e2e-local
```

### Manual Test Run
```bash
cd test/e2e
go test -v ./...
```

## Test Structure

- `operator_test.go` - Basic operator functionality
- `advanced_test.go` - Complex scenarios (selectors, templates)
- `run.sh` - Test runner script

## Environment Variables

- `KUBECONFIG` - Path to kubeconfig (default: ~/.kube/config)
- `OPERATOR_IMAGE` - Operator image to test (default: k8s-acl-operator:latest)
- `TEST_TIMEOUT` - Test timeout (default: 10m)

## Test Scenarios

### Basic Flow
- Create NamespaceRBACConfig
- Create matching/non-matching namespaces
- Verify RBAC resource creation
- Test cleanup

### Advanced
- Complex namespace selectors
- Template variable processing
- Exclusion rules
- Multiple configurations