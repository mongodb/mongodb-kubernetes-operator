# MongoDB Kubernetes Operator 0.6.1

## Kubernetes Operator

- Bug fixes
  - when deleting MongoDB Resource cleanup related resources (k8s services, secrets).
  - fixed an issue where the operator would reconcile based on events emitted by itself in certain situations.
## MongoDB Agent ReadinessProbe

- Changes
  - Readiness probe now patches pod annotations rather than overwriting them.

## Updated Image Tags

- mongodb-kubernetes-operator:0.6.1
- mongodb-kubernetes-readinessprobe:1.0.4

_All the images can be found in:_

https://quay.io/mongodb
