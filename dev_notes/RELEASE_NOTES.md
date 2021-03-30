*(Please use the [release template](release-notes-template.md) as the template for this document)*
<!-- Next release -->

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

<!-- Past Releases -->
