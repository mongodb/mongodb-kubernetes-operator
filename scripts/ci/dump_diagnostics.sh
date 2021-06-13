#!/bin/sh

mkdir -p diagnostics/secrets

namespace="$1"

echo "Dumping CRD"
kubectl get crd mongodbcommunity.mongodbcommunity.mongodb.com -o yaml > diagnostics/crd.yaml || true

# dump logs for every container in every pod in the given namespace
for pod_name in $(kubectl get pod -n "${namespace}" --output=jsonpath={.items..metadata.name}); do
  for container_name in $(kubectl get pods -n "${namespace}" "${pod_name}" -o jsonpath='{.spec.containers[*].name}'); do
      echo "Writing log file for pod ${pod_name} - container ${container_name} to diagnostics/${pod_name}-${container_name}.log"
      kubectl logs -n "${namespace}" "${pod_name}" -c "${container_name}" > "diagnostics/${pod_name}-${container_name}.log" || true;
  done
done

# dump information about MongoDBCommunity resources and statefulsets.
for mdbc_name in $(kubectl get mongodbcommunity -n "${namespace}" --output=jsonpath={.items..metadata.name}); do
    echo "Writing MongoDBCommunity describe"
    kubectl describe mongodbcommunity "${mdbc_name}" -n "${namespace}" > "diagnostics/${mdbc_name}-mongodbcommunity.txt"
    echo "Writing yaml output for MongoDBCommunity ${mdbc_name}"
    kubectl get mongodbcommunity "${mdbc_name}" -n "${namespace}" -o yaml > "diagnostics/${mdbc_name}-mongodbcommunity.yaml"
    echo "Writing describe output for StatefulSet ${mdbc_name}"
    kubectl describe sts "${mdbc_name}" -n "${namespace}" > "diagnostics/${mdbc_name}-statefulset.txt"
    echo "Writing yaml output for StatefulSet ${mdbc_name}"
    kubectl get sts "${mdbc_name}" -n "${namespace}" -o yaml > "diagnostics/${mdbc_name}-statefulset.yaml"

    echo "Writing Automation Config Secret"
    kubectl get secret "${mdbc_name}-config" -o jsonpath='{ .data.cluster-config\.json}' | base64 -d | jq > "diagnostics/secrets/${mdbc_name}-config.json"
done



# dump information about relevant secrets.
# Skip service account tokens, and also skip the Automation Config as this is handled as a special case above.
for secret in $(kubectl get secret -n "${namespace}" --output=jsonpath={.items..metadata.name}); do
  if ! echo "${secret}" | grep -qE "token|-config"; then
    echo "Dumping secret ${secret}"
    kubectl get secret "${secret}" -o json | jq -r '.data | with_entries(.value |= @base64d)' > "diagnostics/secrets/${secret}.json"
  else
      echo "Skipping skipping ${secret}"
  fi
done
