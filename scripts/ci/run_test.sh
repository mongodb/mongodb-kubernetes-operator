#!/usr/bin/env bash

KUBERNETES_SERVICE_HOST="$(kubectl get service kubernetes -o jsonpath='{.spec.clusterIP }')"
temp=$(mktemp)
cat ${KUBECONFIG} | sed "s/server: https.*/server: http:\/\/${KUBERNETES_SERVICE_HOST}/g" > ${temp}
contents="$(cat ${temp})"
kubectl create cm kube-config --from-literal=kubeconfig="${contents}"
rm ${temp}

kubectl apply -f test/replica_set_test.yaml

while [[ $(kubectl get pods -l app=operator-sdk-test -o 'jsonpath={..status.conditions[?(@.type=="Completed")].status}') != "True" ]]; do echo "waiting for tests to complete..." && sleep 1; done