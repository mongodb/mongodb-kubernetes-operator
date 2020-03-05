#!/bin/sh

kubectl apply -f deploy/crds/*crd.yaml -n default
kubectl apply -f deploy -n default

# TODO: Temporary mechanism to update the image of the deployment before a templating mechanism is in place
kubectl set image deployment/mongodb-kubernetes-operator mongodb-kubernetes-operator=quay.io/mongodb/community-operator-dev:${version_id}
