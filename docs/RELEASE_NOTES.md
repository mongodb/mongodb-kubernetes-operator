# MongoDB Kubernetes Operator 0.7.1

## Kubernetes Operator

- Changes
  - MongoDB database of the statefulSet is managed using distinct Role, ServiceAccount and RoleBinding.
  - TLS Secret can also contain a single "tls.pem" entry, containing the concatenation of the certificate and key
    - If a TLS secret contains all of "tls.key", "tls.crt" and "tls.pem" entries, the operator will raise an error if the "tls.pem" one is not equal to the concatenation of "tls.crt" with "tls.key"
  - Readinessprobe reports MongoDB running as Arbitrer as _Running_ & _Healthy_.
  - The `CLUSTER_DOMAIN` environment variable can be set on the Operator Pod, to configure the Kubernetes cluster's Domain,
    in case this one differs from the default `cluster.local`.

## MongoDBCommunity Resource
* Changes
* Specifying `spec.additionalMongodConfig.storage.dbPath` will now be respected correctly. 


## Updated Image Tags

- mongodb-kubernetes-operator:0.7.1

_All the images can be found in:_

https://quay.io/mongodb
