# Kubernetes RBAC Operator

A Kubernetes operator that automatically manages RBAC resources (Roles, ClusterRoles, RoleBindings, ClusterRoleBindings) based on namespace lifecycle events and custom resource configurations.

## Features

- **Flexible Namespace Selection**: Support for regex patterns, labels, annotations, and explicit lists
- **Template-Based RBAC**: Go template system for dynamic RBAC resource generation
- **Conflict Resolution**: Multiple merge strategies for handling overlapping configurations
- **Resource Cleanup**: Automatic cleanup of orphaned cluster-scoped resources
- **Status Tracking**: Comprehensive status reporting and observability
- **Event-Driven**: Responds to namespace creation, deletion, and updates

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.25+)
- kubectl configured to access your cluster
- Go 1.21+ (for development)

### Installation

1. Install the CRD:
```bash
kubectl apply -f config/crd/namespacerbacconfigs.yaml
```

2. Install the operator:
```bash
kubectl apply -f deploy/
```

3. Create a sample configuration:
```bash
kubectl apply -f config/samples/dev-team-rbac.yaml
```

## Configuration

The operator uses `NamespaceRBACConfig` custom resources to define RBAC templates and namespace selection criteria.

### Basic Example

```yaml
apiVersion: rbac.operator.io/v1
kind: NamespaceRBACConfig
metadata:
  name: dev-team-rbac
spec:
  namespaceSelector:
    nameRegex: "^dev-.*"
    annotations:
      "team": "platform"
  
  rbacTemplates:
    roles:
    - name: "developer-{{.Namespace.Name}}"
      rules:
      - apiGroups: [""]
        resources: ["pods", "services"]
        verbs: ["get", "list", "create", "update", "patch", "delete"]
    
    roleBindings:
    - name: "developers-{{.Namespace.Name}}"
      roleRef:
        kind: "Role"
        name: "developer-{{.Namespace.Name}}"
      subjects:
      - kind: "Group"
        name: "dev-team"
        apiGroup: "rbac.authorization.k8s.io"
```

### Template Variables

The following variables are available in templates:

- `{{.Namespace.Name}}` - Name of the target namespace
- `{{.Namespace.Labels.key}}` - Access to namespace labels
- `{{.Namespace.Annotations.key}}` - Access to namespace annotations
- `{{.CRD.Name}}` - Name of the NamespaceRBACConfig
- `{{.Config.Naming.Prefix}}` - Configured naming prefix
- `{{.CustomVars.key}}` - Custom variables from templateVariables

## Development

### Prerequisites

- Go 1.21+
- Docker
- kubectl
- kubebuilder (optional, for scaffolding)

### Building

```bash
# Build the manager binary
make build

# Build and push Docker image
make docker-build docker-push IMG=<registry>/k8s-acl-operator:tag

# Deploy to cluster
make deploy IMG=<registry>/k8s-acl-operator:tag
```

### Running Tests

```bash
# Run unit tests
make test

# Run end-to-end tests
make test-e2e
```

### Local Development

```bash
# Install CRDs
make install

# Run locally (requires kubeconfig)
make run
```

## Architecture

The operator consists of two main controllers:

1. **NamespaceRBACConfig Controller**: Watches for changes to NamespaceRBACConfig resources
2. **Namespace Controller**: Watches for namespace creation/deletion events

When a namespace event occurs, the operator:

1. Evaluates all NamespaceRBACConfig resources against the namespace
2. Applies matching RBAC templates with variable substitution
3. Creates/updates/deletes RBAC resources as needed
4. Updates status fields with current state

## Configuration Options

### Namespace Selection

- `nameRegex`: Regular expression for namespace names
- `annotations`: Required annotations on namespaces
- `labels`: Required labels on namespaces
- `includeNamespaces`: Explicit list of namespaces to include
- `excludeNamespaces`: Explicit list of namespaces to exclude

### Merge Strategies

- `merge` (default): Combine rules from multiple configurations
- `replace`: Last configuration wins
- `ignore`: Skip if resource already exists

### Cleanup Behavior

- `deleteOrphanedClusterResources`: Clean up unused cluster-scoped resources
- `gracePeriodSeconds`: Grace period before deletion

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

For questions and support, please open an issue in the GitHub repository.
