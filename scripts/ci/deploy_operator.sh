#!/bin/sh

kubectl apply -f deploy/crds/*crd.yaml -n default
kubectl apply -f deploy -n default