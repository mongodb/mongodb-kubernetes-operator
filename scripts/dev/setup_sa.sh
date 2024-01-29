#!/usr/bin/env bash

service_accounts=$(kubectl get serviceaccounts -n "${NAMESPACE}" -o jsonpath='{.items[*].metadata.name}')

for service_account in $service_accounts; do
  kubectl patch serviceaccount "$service_account" -n "${NAMESPACE}" -p "{\"imagePullSecrets\": [{\"name\": \"image-registries-secret\"}]}"
done