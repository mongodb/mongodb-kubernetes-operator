#!/usr/bin/env bash
set -Eeou pipefail

docker_file=$(mktemp)
python docker/dockerfile_generator.py operator >"${docker_file}"
docker build . -t "${REPO_URL}/mongodb-kubernetes-operator" -f "${docker_file}" && docker push "${REPO_URL}/mongodb-kubernetes-operator"
rm "${docker_file}"

echo "Deploying Operator"
kubectl apply -f deploy/crds/*crd.yaml
kubectl apply -f deploy

echo "Waiting for Deplment to be available"
kubectl wait -f deploy/operator.yaml --for condition=available

echo "Operator is deployed!"
