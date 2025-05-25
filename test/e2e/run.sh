#!/bin/bash

set -e

# E2E Test Runner for k8s-acl-operator

KUBECONFIG=${KUBECONFIG:-~/.kube/config}
OPERATOR_IMAGE=${OPERATOR_IMAGE:-"k8s-acl-operator:latest"}
TEST_TIMEOUT=${TEST_TIMEOUT:-"10m"}

echo "Running E2E tests..."
echo "Kubeconfig: $KUBECONFIG"
echo "Operator Image: $OPERATOR_IMAGE"

# Check if cluster is accessible
if ! kubectl cluster-info > /dev/null 2>&1; then
    echo "Error: Cannot access Kubernetes cluster"
    exit 1
fi

# Install CRDs
echo "Installing CRDs..."
kubectl apply -f config/crd/

# Deploy operator
echo "Deploying operator..."
kubectl apply -f deploy/manifests/

# Wait for operator to be ready
echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available deployment/k8s-acl-operator-controller-manager \
    -n k8s-acl-operator-system --timeout=300s

# Run tests
echo "Running E2E tests..."
cd test/e2e
go test -v -timeout=$TEST_TIMEOUT ./...

echo "E2E tests completed successfully!"
