#!/usr/bin/env bash
set -Eeou pipefail

# create roles and service account required for the test runner
kubectl apply -f deploy/testrunner
kubectl delete pod test-runner --ignore-not-found

test=${1}
echo "Running Test: ${test}"
# start the test runner pod
kubectl run test-runner --generator=run-pod/v1 \
  --restart=Never \
  --image-pull-policy=Always \
  --image="${REPO_URL}/test-runner" \
  --serviceaccount=test-runner \
  --command -- ./runner --operatorImage "${REPO_URL}/mongodb-kubernetes-operator" --testImage "${REPO_URL}/e2e" --test=${test}

kubectl wait --for=condition=Ready pod -l run=test-runner --timeout=600s
kubectl logs -f -l run=test-runner
