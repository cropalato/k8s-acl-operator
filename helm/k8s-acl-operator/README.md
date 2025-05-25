# K8s ACL Operator Helm Chart

Helm chart for deploying the Kubernetes RBAC Operator that automatically manages RBAC resources based on namespace lifecycle events.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+

## Installing the Chart

```bash
# Add your chart repository (if published)
helm repo add k8s-acl-operator https://your-charts-repo.com
helm repo update

# Or install from local source
helm install k8s-acl-operator ./helm/k8s-acl-operator
```

## Configuration

### Basic Installation

```bash
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --set image.repository=your-registry/k8s-acl-operator \
  --set image.tag=v0.1.0
```

### With Sample Configurations

```bash
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --set samples.enabled=true
```

## Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `k8s-acl-operator` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `namespace.create` | Create namespace | `true` |
| `namespace.name` | Namespace name | `k8s-acl-operator-system` |
| `serviceAccount.create` | Create service account | `true` |
| `operator.leaderElection` | Enable leader election | `true` |
| `rbacProxy.enabled` | Enable RBAC proxy | `true` |
| `samples.enabled` | Deploy sample configs | `false` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |

## Advanced Configuration

### Custom Image

```yaml
# values.yaml
image:
  repository: myregistry.com/k8s-acl-operator
  tag: "v1.0.0"
  pullPolicy: Always

imagePullSecrets:
  - name: my-registry-secret
```

### Resource Configuration

```yaml
# values.yaml
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

rbacProxy:
  resources:
    limits:
      cpu: 200m
      memory: 64Mi
```

### Disable RBAC Proxy

```yaml
# values.yaml
rbacProxy:
  enabled: false

metrics:
  secure: false
```

## Usage Examples

### 1. Install with Custom Namespace

```bash
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --create-namespace \
  --namespace custom-operator-ns \
  --set namespace.create=false
```

### 2. Install with Sample Configurations

```bash
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --set samples.enabled=true \
  --set image.tag=v0.1.0
```

### 3. Production Deployment

```bash
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --set image.repository=prod-registry.com/k8s-acl-operator \
  --set image.tag=v1.2.3 \
  --set replicaCount=2 \
  --set operator.leaderElection=true \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=256Mi
```

## Upgrading

```bash
# Upgrade to new version
helm upgrade k8s-acl-operator ./helm/k8s-acl-operator \
  --set image.tag=v0.2.0

# Upgrade with new values
helm upgrade k8s-acl-operator ./helm/k8s-acl-operator \
  --values my-values.yaml
```

## Uninstalling

```bash
# Remove the operator
helm uninstall k8s-acl-operator

# Remove CRDs (manual step)
kubectl delete crd namespacerbacconfigs.rbac.operator.io
```

## Monitoring

### Accessing Metrics

```bash
# Port forward to access metrics
kubectl port-forward -n k8s-acl-operator-system \
  svc/k8s-acl-operator-controller-manager-metrics-service 8443:8443

# Get metrics
curl -k https://localhost:8443/metrics
```

### Check Operator Status

```bash
# View deployment
kubectl get deployment -n k8s-acl-operator-system

# Check logs
kubectl logs -f deployment/k8s-acl-operator-controller-manager \
  -n k8s-acl-operator-system

# View NamespaceRBACConfigs
kubectl get namespacerbacconfigs
```

## Troubleshooting

### Common Issues

1. **CRD Already Exists**
   ```bash
   # Check existing CRDs
   kubectl get crd namespacerbacconfigs.rbac.operator.io
   
   # Remove if needed
   kubectl delete crd namespacerbacconfigs.rbac.operator.io
   ```

2. **RBAC Permissions**
   ```bash
   # Check ClusterRole
   kubectl get clusterrole | grep k8s-acl-operator
   
   # Verify ClusterRoleBinding
   kubectl get clusterrolebinding | grep k8s-acl-operator
   ```

3. **Image Pull Issues**
   ```bash
   # Check pod status
   kubectl get pods -n k8s-acl-operator-system
   
   # Describe pod for events
   kubectl describe pod <pod-name> -n k8s-acl-operator-system
   ```

### Debug Mode

```bash
# Install with debug logging
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --set operator.logLevel=debug
```

## Development

### Testing Chart Changes

```bash
# Dry run
helm install k8s-acl-operator ./helm/k8s-acl-operator --dry-run

# Template rendering
helm template k8s-acl-operator ./helm/k8s-acl-operator

# Lint chart
helm lint ./helm/k8s-acl-operator
```

### Values Validation

```bash
# Validate values file
helm install k8s-acl-operator ./helm/k8s-acl-operator \
  --values my-values.yaml \
  --dry-run --debug
```

## Integration

### ArgoCD

```yaml
# application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: k8s-acl-operator
spec:
  source:
    repoURL: https://github.com/yourusername/k8s-acl-operator
    path: helm/k8s-acl-operator
    helm:
      values: |
        image:
          tag: v1.0.0
        samples:
          enabled: true
```

### Flux

```yaml
# helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: k8s-acl-operator
spec:
  chart:
    spec:
      chart: ./helm/k8s-acl-operator
      sourceRef:
        kind: GitRepository
        name: k8s-acl-operator
  values:
    image:
      tag: v1.0.0
```
