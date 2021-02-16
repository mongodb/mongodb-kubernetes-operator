#!/bin/sh

kind create cluster --kubeconfig "${KUBECONFIG}"

echo "Creating CRDs"
kubectl apply -f config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml
