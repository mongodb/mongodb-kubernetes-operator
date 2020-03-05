#!/usr/bin/env bash


# This is a temporary fix to correct the KUBECONFIG file that gets mounted.
# When the KUBECONFIG file is created by kind, it points to localhost. When inside
# the cluster, we need to set this value to be .spec.clusterIP instead
# See: https://github.com/operator-framework/operator-sdk/issues/2618
KUBERNETES_SERVICE_HOST="$(kubectl get service kubernetes -o jsonpath='{.spec.clusterIP }')"
temp=$(mktemp)
cat ${KUBECONFIG} | sed "s/server: https.*/server: https:\/\/${KUBERNETES_SERVICE_HOST}/g" > ${temp}
contents="$(cat ${temp})"
kubectl create cm kube-config --from-literal=kubeconfig="${contents}"
rm ${temp}

sed -i "s|E2E_TEST_IMAGE|quay.io/mongodb/community-operator-e2e:${revision}|g" test/replica_set_test.yaml
kubectl apply -f test/replica_set_test.yaml

echo "Waiting for test application to be deployed"
kubectl wait --for=condition=Ready pod -l app=operator-sdk-test --timeout=500s
echo "Test pod is ready to begin"

# The test will have fully finished when tailing logs finishes
kubectl logs -f -l app=operator-sdk-test

result="$(kubectl get pod -l app=operator-sdk-test -o jsonpath='{ .items[0].status.phase }')"
if [[ ${result} != "Succeeded" ]]; then
  exit 1
fi
