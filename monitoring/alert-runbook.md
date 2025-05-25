# RBAC Operator Alert Runbook

This document provides response procedures for RBAC Operator alerts. Each alert includes severity, investigation steps, and resolution actions.

## Alert Response Matrix

| Alert | Severity | Response Time | Escalation |
|-------|----------|---------------|------------|
| RBACOperatorDown | Critical | Immediate | Yes |
| RBACOperatorHighErrorRate | Critical | 5 minutes | Yes |
| RBACOperatorComponentUnhealthy | Critical | 5 minutes | Yes |
| RBACOperatorStaleReconciliation | Critical | 10 minutes | Yes |
| Resource/Template Failures | Warning | 15 minutes | If persistent |
| Performance Issues | Warning | 30 minutes | If degrading |
| Activity/Drift Alerts | Info | Best effort | No |

---

## Critical Alerts

### RBACOperatorDown
**Impact**: No RBAC management, security policies not enforced
**Urgency**: Immediate action required

#### Investigation
```bash
# Check pod status
kubectl get pods -n rbac-operator-system

# Check recent events
kubectl describe pod -n rbac-operator-system <pod-name>

# Check logs
kubectl logs -n rbac-operator-system <pod-name> --tail=100
```

#### Resolution
1. **Pod Issues**: Restart deployment
   ```bash
   kubectl rollout restart deployment/rbac-operator -n rbac-operator-system
   ```

2. **Resource Constraints**: Check CPU/memory limits
   ```bash
   kubectl top pods -n rbac-operator-system
   ```

3. **Image Issues**: Verify image availability and tags

4. **Node Issues**: Check node status and resources

### RBACOperatorHighErrorRate
**Impact**: RBAC policies may not be applied correctly
**Urgency**: Investigate within 5 minutes

#### Investigation
```bash
# Check error patterns in logs
kubectl logs -n rbac-operator-system deployment/rbac-operator | grep -i error

# Check specific config causing errors
kubectl get namespacerbacconfigs -o wide

# Review recent reconciliation status
kubectl describe namespacerbacconfig <config-name>
```

#### Resolution
1. **Validation Errors**: Fix NamespaceRBACConfig spec
2. **API Errors**: Check cluster API server health
3. **Permission Errors**: Verify operator RBAC permissions
4. **Template Errors**: Review template syntax in configs

### RBACOperatorComponentUnhealthy
**Impact**: Degraded functionality in specific components
**Urgency**: Investigate within 5 minutes

#### Investigation
```bash
# Check health endpoint
kubectl port-forward -n rbac-operator-system svc/rbac-operator 8081:8081
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz

# Check component-specific logs
kubectl logs -n rbac-operator-system deployment/rbac-operator | grep -E "(reconciler|rbac_manager|health_checker)"
```

#### Resolution
1. **Reconciler Issues**: Check for stuck reconciliation loops
2. **RBAC Manager Issues**: Verify Kubernetes API connectivity
3. **Health Checker Issues**: Usually resolves automatically

### RBACOperatorStaleReconciliation
**Impact**: RBAC changes not being processed
**Urgency**: Investigate within 10 minutes

#### Investigation
```bash
# Check last reconciliation times
kubectl get namespacerbacconfigs -o custom-columns=NAME:.metadata.name,LAST-RECONCILE:.status.conditions[?(@.type=="Ready")].lastTransitionTime

# Look for stuck reconciliations
kubectl logs -n rbac-operator-system deployment/rbac-operator | grep -i "reconcil"

# Check for blocking conditions
kubectl describe namespacerbacconfig <config-name>
```

#### Resolution
1. **Controller Issues**: Restart operator
2. **Resource Locks**: Clear finalizers if needed
3. **API Throttling**: Check rate limits

---

## Warning Alerts

### RBACOperatorResourceOperationFailures
**Impact**: Specific RBAC resources not created/updated
**Urgency**: Investigate within 15 minutes

#### Investigation
```bash
# Check which resource types are failing
kubectl logs -n rbac-operator-system deployment/rbac-operator | grep -E "(role|clusterrole|binding)" | grep -i error

# Verify target namespaces exist
kubectl get namespaces

# Check for permission issues
kubectl auth can-i create roles --as=system:serviceaccount:rbac-operator-system:rbac-operator
```

#### Resolution
1. **Permission Issues**: Update operator ClusterRole
2. **Resource Conflicts**: Review merge strategies
3. **Namespace Issues**: Ensure target namespaces exist

### RBACOperatorTemplateProcessingErrors
**Impact**: Template expansion failures prevent RBAC creation
**Urgency**: Investigate within 15 minutes

#### Investigation
```bash
# Check template syntax in configs
kubectl get namespacerbacconfigs -o yaml | grep -A 10 -B 10 "template"

# Look for template-specific errors
kubectl logs -n rbac-operator-system deployment/rbac-operator | grep -i template
```

#### Resolution
1. **Syntax Errors**: Fix Go template syntax
2. **Missing Variables**: Add required custom variables
3. **Invalid References**: Verify namespace labels/annotations exist

### RBACOperatorSlowReconciliation
**Impact**: Delayed RBAC policy application
**Urgency**: Monitor, investigate if persistent

#### Investigation
```bash
# Check cluster resource usage
kubectl top nodes
kubectl top pods -A

# Review large configs
kubectl get namespacerbacconfigs -o custom-columns=NAME:.metadata.name,NAMESPACES:.status.appliedNamespaces

# Check for API server latency
kubectl get --raw=/metrics | grep apiserver_request_duration
```

#### Resolution
1. **Resource Constraints**: Scale up nodes or increase limits
2. **Large Configs**: Consider splitting complex configurations
3. **API Latency**: Investigate cluster performance

---

## Info Alerts

### RBACOperatorNoActivity
**Context**: No reconciliation activity detected
**Action**: Verify this is expected behavior

#### Investigation
- Check if cluster changes are expected
- Verify configs are properly configured
- Ensure namespaces match selectors

### RBACOperatorHighConflictResolution
**Context**: Frequent merge strategy usage
**Action**: Review configuration overlap

#### Investigation
- Identify overlapping NamespaceRBACConfigs
- Consider consolidating or adjusting selectors
- Review merge strategy choices

---

## Emergency Procedures

### Complete Operator Failure
1. **Immediate**: Document current RBAC state
   ```bash
   kubectl get roles,clusterroles,rolebindings,clusterrolebindings -A -l rbac.operator.io/owned-by=namespace-rbac-operator > rbac-backup.yaml
   ```

2. **Temporary**: Manual RBAC management if critical
3. **Recovery**: Restore operator and validate state

### Rollback Procedure
```bash
# Rollback to previous version
kubectl rollout undo deployment/rbac-operator -n rbac-operator-system

# Verify rollback
kubectl rollout status deployment/rbac-operator -n rbac-operator-system

# Check functionality
kubectl get namespacerbacconfigs
```

### Clean Slate Recovery
```bash
# Delete all managed resources
kubectl delete clusterroles,clusterrolebindings -l rbac.operator.io/owned-by=namespace-rbac-operator

# Restart operator
kubectl rollout restart deployment/rbac-operator -n rbac-operator-system

# Monitor reconciliation
kubectl logs -f -n rbac-operator-system deployment/rbac-operator
```

---

## Useful Commands

### Metrics Collection
```bash
# Port forward metrics
kubectl port-forward -n rbac-operator-system svc/rbac-operator 8080:8080

# Get metrics
curl http://localhost:8080/metrics | grep rbac_operator
```

### Debug Information
```bash
# Full operator status
kubectl get pods,svc,configmap,secret -n rbac-operator-system

# All RBAC configs and status
kubectl get namespacerbacconfigs -o yaml > debug-configs.yaml

# Managed resources
kubectl get roles,clusterroles,rolebindings,clusterrolebindings -A -l rbac.operator.io/owned-by=namespace-rbac-operator
```

### Performance Tuning
```bash
# Increase operator resources
kubectl patch deployment rbac-operator -n rbac-operator-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","resources":{"requests":{"cpu":"200m","memory":"256Mi"},"limits":{"cpu":"500m","memory":"512Mi"}}}]}}}}'

# Adjust reconciliation rate
kubectl patch deployment rbac-operator -n rbac-operator-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","args":["--health-probe-bind-address=:8081","--metrics-bind-address=:8080","--leader-elect","--reconcile-frequency=30s"]}]}}}}'
```

---

## Escalation Contacts

- **L1 Support**: Platform team on-call
- **L2 Support**: Kubernetes team lead  
- **L3 Support**: RBAC operator maintainers

## Related Documentation

- [RBAC Operator Design](../docs/development.md)
- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Prometheus Metrics Reference](./README.md)
