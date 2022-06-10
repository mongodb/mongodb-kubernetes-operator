# MongoDB Kubernetes Operator 0.7.4

## Kubernetes Operator

- Bug fixes
  - The names of connection string secrets generated for configured users are RFC1123 validated.
- Changes
  - Support for changing port number in running cluster.
  
## MongoDBCommunity Resource

- Changes
  - Adds an optional field `users[i].connectionStringSecretName` for deterministically setting the name of the connection string secret created by the operator for every configured user.

- Bug fixes
  - Allows for *arbiters* to be set using `spec.arbiters` attribute. Fixes a condition where *arbiters* could not be added to the Replica Set.

## Updated Image Tags

- mongodb-kubernetes-operator:0.7.3
- mongodb-agent:11.12.0.7388-1
- mongodb-kubernetes-readinessprobe:1.0.8
- mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.4

_All the images can be found in:_

https://quay.io/mongodb
