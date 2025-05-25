# Development Guide

## Overview

This guide covers development workflows for the k8s-acl-operator project.

## Prerequisites

- Go 1.21+
- Docker
- kubectl
- Access to a Kubernetes cluster (for testing)

## Development Workflow

### 1. Setting up the development environment

```bash
# Clone the repository
git clone <repository-url>
cd k8s-acl-operator

# Set up development environment
make dev-setup
```

### 2. Making changes

```bash
# Run tests
make test

# Run linting
make lint

# Format code
make fmt

# Run all checks
make check
```

### 3. Testing locally

```bash
# Install CRDs
make install

# Run the operator locally
make run

# In another terminal, apply test configurations
make examples
```

### 4. Building and deploying

```bash
# Build the binary
make build

# Build container image
make docker-build IMG=your-registry/k8s-acl-operator:tag

# Push container image
make docker-push IMG=your-registry/k8s-acl-operator:tag

# Deploy to cluster
make deploy IMG=your-registry/k8s-acl-operator:tag
```

## Code Structure

### Controllers

- `pkg/controller/namespacerbacconfig/` - Main controller for NamespaceRBACConfig resources
- `pkg/controller/namespace/` - Controller that watches namespace events

### APIs

- `pkg/apis/rbac/v1/` - API types and registration for the custom resources

### Core Logic

- `pkg/rbac/` - RBAC resource management logic
- `pkg/template/` - Template processing engine
- `pkg/utils/` - Utility functions

## Testing

### Unit Tests

```bash
# Run unit tests
make test

# Run quick tests (without envtest)
make quick-test
```

### End-to-End Tests

```bash
# TODO: Implement e2e tests
```

## Debugging

### Local Development

When running locally with `make run`, the operator will:

1. Connect to your current kubeconfig context
2. Watch for NamespaceRBACConfig and Namespace events
3. Apply RBAC resources based on the configurations
4. Log activity to stdout

### In-Cluster Debugging

```bash
# View operator logs
kubectl logs -n k8s-acl-operator-system deployment/k8s-acl-operator-controller-manager

# Check operator status
kubectl get pods -n k8s-acl-operator-system

# View applied configurations
kubectl get namespacerbacconfigs

# Check created RBAC resources
kubectl get roles,rolebindings,clusterroles,clusterrolebindings -l rbac.operator.io/owned-by=namespace-rbac-operator
```

## Configuration Examples

See the `config/samples/` directory for example configurations:

- `dev-team-rbac.yaml` - Example for development team access
- `admin-rbac.yaml` - Example for admin access

## Common Issues

### CRDs not installed

```bash
make install
```

### Permission errors

Ensure the operator has proper RBAC permissions (see `deploy/rbac.yaml`)

### Template errors

Check the operator logs for template processing errors. Common issues:
- Syntax errors in Go templates
- Missing template variables
- Invalid namespace selectors

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make check` to ensure all tests pass
6. Submit a pull request
