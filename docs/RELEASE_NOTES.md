# MongoDB Kubernetes Operator 0.7.1

## Kubernetes Operator

- Changes
  - MongoDB database of the statefulSet is managed using distinct Role, ServiceAccount and RoleBinding.
  - TLS Secret can also contain a single "tls.pem" entry, containing the concatenation of the certificate and key
    - If a TLS secret contains all three entries, the operator will raise an error if the tls.pem entry is not equal to the concatenation of the tls.crt and tls.key entries
## Updated Image Tags

- mongodb-kubernetes-operator:0.7.1

_All the images can be found in:_

https://quay.io/mongodb
