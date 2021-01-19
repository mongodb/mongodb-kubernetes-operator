#!/bin/sh

kind create cluster --kubeconfig "${KUBECONFIG}"

echo "Creating CRDs"
kubectl apply -f deploy/crds/*crd.yaml
