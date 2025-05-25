# RBAC Operator Monitoring

Comprehensive monitoring setup for the RBAC Operator including Prometheus metrics, Grafana dashboards, and alert runbooks.

## Components

- **Grafana Dashboard** (`grafana-dashboard.json`) - Visual monitoring with 13 panels
- **Prometheus Alerts** (`prometheus-alerts.yaml`) - Critical, warning, and info level alerts
- **Alert Runbook** (`alert-runbook.md`) - Response procedures for each alert

## Quick Setup

### 1. Install Grafana Dashboard
```bash
# Import via UI or API
curl -X POST http://grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @monitoring/grafana-dashboard.json
```

### 2. Deploy Prometheus Alerts
```bash
# Add to Prometheus configuration
kubectl apply -f monitoring/prometheus-alerts.yaml

# Or include in prometheus.yml
rule_files:
  - "rbac-operator-alerts.yaml"
```

### 3. Configure Scraping
Add to Prometheus `scrape_configs`:
```yaml
- job_name: 'rbac-operator'
  static_configs:
  - targets: ['rbac-operator.rbac-operator-system:8080']
  metrics_path: /metrics
  scrape_interval: 30s
```

## Dashboard Panels

| Panel | Purpose |
|-------|---------|
| Overview | Active configs, managed namespaces/resources |
| Success Rate | Reconciliation success percentage |
| Error Tracking | Error rates by type and component |
| Performance | Duration percentiles and processing times |
| Resources | Managed resource counts and operations |
| Health Status | Component health monitoring |

## Key Metrics

- `rbac_operator_reconciliation_total` - Success/error counts
- `rbac_operator_reconciliation_duration_seconds` - Performance tracking
- `rbac_operator_managed_resources_total` - Resource inventory
- `rbac_operator_health_status` - Component health

## Alert Severity

**Critical**: Immediate response required
- Operator down, high error rates (>10%), stale reconciliation

**Warning**: Investigate within 15-30 minutes  
- Resource failures, template errors, slow performance

**Info**: Monitor trends
- No activity, resource drift, high conflicts

## Useful Queries

### Success Rate
```promql
rate(rbac_operator_reconciliation_total{result="success"}[5m]) / 
rate(rbac_operator_reconciliation_total[5m]) * 100
```

### 95th Percentile Duration
```promql
histogram_quantile(0.95, rate(rbac_operator_reconciliation_duration_seconds_bucket[5m]))
```

### Error Rate by Type
```promql
rate(rbac_operator_reconciliation_errors_total[5m])
```

## Troubleshooting

Access operator metrics:
```bash
kubectl port-forward -n rbac-operator-system svc/rbac-operator 8080:8080
curl http://localhost:8080/metrics | grep rbac_operator
```

View alert status:
```bash
curl http://prometheus:9090/api/v1/alerts | jq '.data.alerts[] | select(.labels.alertname | startswith("RBACOperator"))'
```

## Alert Response

Follow procedures in `alert-runbook.md` for each alert type. Response times:
- Critical: Immediate to 10 minutes
- Warning: 15-30 minutes  
- Info: Best effort

## Related Documentation

- [Alert Runbook](./alert-runbook.md) - Detailed response procedures
- [Development Guide](../docs/development.md) - Operator architecture
- [RBAC CRD Design](../CRD_DESIGN_FEATURES.md) - Configuration reference
