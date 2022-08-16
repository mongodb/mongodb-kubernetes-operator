# MongoDB Kubernetes Operator 0.7.5

## Upgrade breaking change notice
Versions 0.7.3, 0.7.4 have an issue that breaks deployment of MongoDB replica set when:
* TLS is enabled
* Replica set was deployed using the operator with version <=0.7.2

If above conditions are met, it is strongly advised to upgrade the MongoDB Kubernetes Operator to version 0.7.5 or higher.

## Kubernetes Operator

- Bug fixes
  - Fixed ignoring changes to existing volumes in the StatefulSet, i.e. changes of the volumes' underlying secret. This could cause that TLS enabled MongoDB deployment was not able to locate TLS certificates when upgrading the operator to versions 0.7.3 or 0.7.4.   

- Security fixes
  - The operator, readiness and versionhook binaries are now built with 1.18.5 which addresses security issues.

# MongoDB Kubernetes Operator 0.7.4

## Kubernetes Operator

- Bug fixes
  - The names of connection string secrets generated for configured users are RFC1123 validated.
- Changes
  - Support for changing port number in running cluster.
  - Security Context is now defined on pod level (previously was on container level)
  - Our containers now use the `readOnlyRootFilesystem` setting.

## MongoDBCommunity Resource

- Changes
  - Adds an optional field `users[i].connectionStringSecretName` for deterministically setting the name of the connection string secret created by the operator for every configured user.

- Bug fixes
  - Allows for *arbiters* to be set using `spec.arbiters` attribute. Fixes a condition where *arbiters* could not be added to the Replica Set.

## Updated Image Tags

- mongodb-kubernetes-operator:0.7.4
- mongodb-agent:11.12.0.7388-1
- mongodb-kubernetes-readinessprobe:1.0.9
- mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.4

_All the images can be found in:_

https://quay.io/mongodb
