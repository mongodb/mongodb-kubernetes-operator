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


# create roles and service account required for the test runner
kubectl apply -f deploy/testrunner

# start the test runner pod
kubectl run test-runner --generator=run-pod/v1 \
  --restart=Never \
  --image=quay.io/chatton/test-runner \
  --serviceaccount=test-runner \
  --command -- ./runner  --operatorImage quay.io/mongodb/community-operator-dev:${version_id} --testImage quay.io/mongodb/community-operator-e2e:${version_id}


echo "Test pod is ready to begin"
kubectl wait --for=condition=Ready pod -l run=test-runner --timeout=600s

# The test will have fully finished when tailing logs finishes
kubectl logs -f -l run=test-runner

result="$(kubectl get pod -l run=test-runner -o jsonpath='{ .items[0].status.phase }')"
if [[ ${result} != "Succeeded" ]]; then
  exit 1
fi
