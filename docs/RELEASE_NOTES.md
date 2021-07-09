# MongoDB Kubernetes Operator 0.7.0

## Kubernetes Operator

- Changes
  - Members of a Replica Set can be configured as arbiters.
  - Reduce the number of permissions for operator role.
  - Support SHA-1 as an authentication method.
  - Upgraded `mongodbcommunity.mongodbcommunity.mongodb.com` CRD to `v1` from `v1beta1`
    - Users upgrading their CRD from v1beta1 to v1 need to set: `spec.preserveUnknownFields` to `false` in the CRD file `config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml` before applying the CRD to the cluster.
  - Made service name configurable in mongdb custom resource with statefulSet.spec.serviceName
  - Add Kube-linter as a github actions.

## Updated Image Tags

- mongodb-kubernetes-operator:0.7.0

_All the images can be found in:_

https://quay.io/mongodb
