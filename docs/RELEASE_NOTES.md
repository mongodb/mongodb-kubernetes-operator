# MongoDB Kubernetes Operator 0.7.3

## Kubernetes Operator

- Changes
  - The Operator can correctly scale arbiters up and down. When arbiters are
    enabled (this is, when `spec.arbiters > 0`), a new StatefulSet will be
    created to hold the Pods that will act as arbiters. The new StatefulSet will
    be named `<mongodb-resource>-arb`.
  - Add support for exposing Prometheus metrics from the ReplicaSet
- Bug fixes
  - The operator will watch for changes in the referenced CA certificates as well as server certificates

## MongoDBCommunity Resource

- Changes
  - Exposing Prometheus metrics is now possible by configuring `spec.prometheus`.


## Updated Image Tags

- mongodb-kubernetes-operator:0.7.3
- mongodb-agent:11.12.0.7388-1
- mongodb-kubernetes-readinessprobe:1.0.8
- mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.4

_All the images can be found in:_

https://quay.io/mongodb
