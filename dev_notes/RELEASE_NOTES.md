*(Please use the [release template](release-notes-template.md) as the template for this document)*
<!-- Next release -->
# MongoDB Kubernetes Operator 0.5.3
## Kubernetes Operator

* Breaking Changes
  * A new VolumeClaimTemplate has been added `logs-volume`. When you deploy the operator, if there is an existing StatefulSet the operator will attempt to perform an invalid update. The existing StatefulSet must be deleted before upgrading the operator.
  
  * The user of the mongod and mongodb-agent containers has changed. This means that there will be permissions
    issues when upgrading from an earlier version of the operator. In order to update the permissions in the volume, you can use an init container.

* Upgrade instructions:

  Remove the current operator
  -  `kubectl delete <operator-deployment>`
  Delete the existing StatefulSet
  -   `kubectl delete statefulset <sts-name>`
  Install the new operator
  - follow the regular [installation instruction](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/docs/install-upgrade.md)
  Patch the StatefulSet once it has been created. This will add an init container that will update the permissions of the existing volume.
  - `kubectl patch statefulset <sts-name> --type='json' --patch '[ {"op":"add","path":"/spec/template/spec/initContainers/-", "value": { "name": "change-data-dir-permissions", "image": "busybox", "command": [ "chown", "-R", "2000", "/data" ], "securityContext": { "runAsNonRoot": false, "runAsUser": 0, "runAsGroup":0 }, "volumeMounts": [ { "mountPath": "/data", "name" : "data-volume" } ] } } ]'`
   
* Bug fixes
  * Fixes an issue that prevented the agents from reaching goal state when upgrading minor version of MongoDB.

<!-- Past Releases -->
# MongoDB Kubernetes Operator 0.5.2
## Kubernetes Operator
* Changes
  * Readiness probe has been moved into an init container from the Agent image.
  * Security context is now added when the `MANAGED_SECURITY_CONTEXT` environment variable is not set.
* Bug fixes
  * Removed unnecessary environment variable configuration in the openshift samples.
  * Fixed an issue where the operator would perform unnecessary reconcilliations when Secrets were modified.
  * Fixed an issue where a race condition could cause the deployment to get into a bad state when TLS
    settings when being changed at the same time as a scaling operation was happening.
  * Fixed an issue where the agent pod would panic when running as a non-root user.

## MongoDBCommunity Resource
* Changes
 * Added `spec.security.authentication.ignoreUnknownUsers` field. This value defaults to `true`. When enabled,
   any MongoDB users added through external sources will not be removed.


## Miscellaneous
* Changes
  * Internal code refactorings to allow libraries to be imported into other projects.


 ## Updated Image Tags
 * mongodb-kubernetes-operator:0.5.2
 * mongodb-agent:10.27.0.6772-1
 * mongodb-kubernetes-readinessprobe:1.0.1 [new image]
