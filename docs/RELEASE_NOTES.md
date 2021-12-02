# MongoDB Kubernetes Operator 0.7.3

## Kubernetes Operator

- Changes
  - The Operator can correctly scale arbiters up and down. When arbiters are
    enabled (this is, when `spec.arbiters > 0`), a new StatefulSet will be
    created to hold the Pods that will act as arbiters. The new StatefulSet will
    be named `<mongodb-resource>-arb`.

## MongoDBCommunity Resource

- No changes.


## Updated Image Tags

- mongodb-kubernetes-operator:0.7.3

_All the images can be found in:_

https://quay.io/mongodb
