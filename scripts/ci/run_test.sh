#!/usr/bin/env bash

KUBERNETES_SERVICE_HOST="$(kubectl get service kubernetes -o jsonpath='{.spec.clusterIP }')"
temp=$(mktemp)
cat ${KUBECONFIG} | sed "s/server: https.*/server: http:\/\/${KUBERNETES_SERVICE_HOST}/g" > ${temp}
contents="$(cat ${temp})"
kubectl create cm kube-config --from-literal=kubeconfig="${contents}"
rm ${temp}

kubectl apply -f test/replica_set_test.yaml

kubectl wait --for=condition=ready deployment -l app=operator-sdk-test --timeout=300s
echo "Tests have started running!"

kubectl logs -f -l app=operator-sdk-test

echo "Tests have completed running!"
