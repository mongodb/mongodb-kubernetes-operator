# MongoDB Kubernetes Operator 0.6.2

## Kubernetes Operator

- Changes
  - stability improvements when changing version of MongoDB.
  - increased number of concurrent resources the operator can act on.
  - mongodb will now send its log to stdout by default.
  - changed the default values for `MONGODB_REPO_URL` and `MONGODB_IMAGE` in the operator deployment
  - added the support of arbiters for the replica sets.

## Updated Image Tags

- mongodb-kubernetes-operator:0.6.2

_All the images can be found in:_

https://quay.io/mongodb
