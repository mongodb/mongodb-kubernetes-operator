# MongoDB Kubernetes Operator 0.7.6

## Kubernetes Operator

- Changes
  - `mongodb-kubernetes-operator` image is now rebuilt daily, incorporating updates to system packages and security fixes. The operator binary is built only once during the release process and used without changes in daily rebuild.
  - Improved security by introducing `readOnlyRootFilesystem` property to all deployed containers. This change also introduces a few additional volumes and volume mounts.
  - Improved security by introducing `allowPrivilegeEscalation` set to `false` for all containers.

## Updated Image Tags

- mongodb-kubernetes-operator:0.7.6
- mongodb-agent:12.0.10.7591-1
- mongodb-kubernetes-readinessprobe:1.0.11
- mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.5

_All the images can be found in:_

https://quay.io/mongodb
