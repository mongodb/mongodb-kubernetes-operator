#!/usr/bin/env bash

mkdir -p diagnostics/secrets

namespace="$1"

echo "Dumping CRD"
kubectl get crd mongodbcommunity.mongodbcommunity.mongodb.com -o yaml > diagnostics/crd.yaml

echo "Dumping Pod list"
kubectl get pods > diagnostics/pod-list.txt

echo "Dumping Event list"
kubectl get events --sort-by='.lastTimestamp' -owide > diagnostics/events-list.txt

echo "Dumping yaml Event list"
kubectl  kubectl get events --sort-by='.lastTimestamp'  -oyaml > diagnostics/events-list.yaml

# dump operator deployment information.
for deployment_name in $(kubectl get deployment -n "${namespace}" --output=jsonpath={.items..metadata.name}); do
  echo "Writing Deployment describe for deployment ${deployment_name}"
  kubectl describe deploy "${deployment_name}" > "diagnostics/${deployment_name}.txt"

  echo "Writing Deployment yaml for deployment ${deployment_name}"
  kubectl get deploy "${deployment_name}" -o yaml > "diagnostics/${deployment_name}.yaml"
done

# dump logs for every container in every pod in the given namespace
for pod_name in $(kubectl get pod -n "${namespace}" --output=jsonpath={.items..metadata.name}); do
  echo "Writing Pod describe for pod ${pod_name}"
  kubectl describe pod "${pod_name}" > "diagnostics/${pod_name}.txt"

  echo "Writing Pod yaml for pod ${pod_name}"
  kubectl get pod "${pod_name}" -o yaml > "diagnostics/${pod_name}.yaml"

  # dump agent output
  kubectl cp "${pod_name}":/var/log/mongodb-mms-automation -c mongodb-agent diagnostics/"${pod_name}-mongodb-automation"/

  for container_name in $(kubectl get pods -n "${namespace}" "${pod_name}" -o jsonpath='{.spec.containers[*].name}'); do
      echo "Writing log file for pod ${pod_name} - container ${container_name} to diagnostics/${pod_name}-${container_name}.log"
      kubectl logs -n "${namespace}" "${pod_name}" -c "${container_name}" > "diagnostics/${pod_name}-${container_name}.log";
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
    echo "Writing secret ${secret}"
    kubectl get secret "${secret}" -o json | jq -r '.data | with_entries(.value |= @base64d)' > "diagnostics/secrets/${secret}.json"
  else
    echo "Skipping secret ${secret}"
  fi
done
