# MongoDB Kubernetes Operator 0.13.0

## Dependency updates
 - Updated kubernetes dependencies to 1.30
 - Bumped Go dependency to 1.24
 - Updated packages `crypto`, `net` and `oauth2` to remediate multiple CVEs

## MongoDBCommunity Resource
 - Added support for overriding the ReplicaSet ID  ([#1656](https://github.com/mongodb/mongodb-kubernetes-operator/pull/1656)).

## Improvements
 - Refactored environment variable propagation ([#1676](https://github.com/mongodb/mongodb-kubernetes-operator/pull/1676)).
 - Introduced a linter to limit inappropriate usage of environment variables within the codebase ([#1690](https://github.com/mongodb/mongodb-kubernetes-operator/pull/1690)).

## Security & Dependency Updates
 - **CVE Updates**: Updated packages `crypto`, `net` and `oauth2` to remediate multiple CVEs
 - Upgraded to Go 1.24 and Kubernetes dependencies to 1.30.x .

