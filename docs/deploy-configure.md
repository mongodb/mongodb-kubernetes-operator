# Deploy and Configure a MongoDB Resource #

The [`/deploy/crds`](../deploy/crds) directory contains example MongoDB resources that you can modify and deploy.

## Table of Contents

- [Deploy a Replica Set](#deploy-a-replica-set)
- [Upgrade MongoDB Version & FCV](#upgrade-your-mongodb-resource-version-and-feature-compatibility-version)
- [Deploying on Openshift](#deploying-on-openshift)

## Deploy a Replica Set

To deploy your first replica set:

1. Invoke the following `kubectl` command:
   ```
   kubectl apply -f deploy/crds/mongodb.com_v1_mongodb_cr.yaml --namespace <my-namespace>
   ```
2. Verify that the MongoDB resource deployed:
   ```
   kubectl get mongodb --namespace <my-namespace>
   ```
3. Connect clients to the MongoDB replica set:
   ```
   mongo "mongodb://<service-object-name>.<namespace>.svc.cluster.local:27017/?replicaSet=<replica-set-name>"
   ```
<em>NOTE: You can access the mongodb instance only from within a pod running in the cluster.</em>

## Upgrade your MongoDB Resource Version and Feature Compatibility Version

You can upgrade the major, minor, and/or feature compatibility versions of your MongoDB resource. These settings are configured in your resource definition YAML file.

- To upgrade your resource's major and/or minor versions, set the `spec.version` setting to the desired MongoDB version.

- To modify your resource's [feature compatibility version](https://docs.mongodb.com/manual/reference/command/setFeatureCompatibilityVersion/), set the `spec.featureCompatibilityVersion` setting to the desired version.

If you update `spec.version` to a later version, consider setting `spec.featureCompatibilityVersion` to the current working MongoDB version to give yourself the option to downgrade if necessary. To learn more about feature compatibility, see [`setFeatureCompatibilityVersion`](https://docs.mongodb.com/manual/reference/command/setFeatureCompatibilityVersion/) in the MongoDB Manual.

## Deploying on OpenShift

If you want to deploy the operator on OpenShift you will have to provide the environment variable `MANAGED_SECURITY_CONTEXT` set to `true` for both the mongodb and mongodb agent containers, as well as the operator deployment.

See [here](../deploy/crds/mongodb.com_v1_mongodb_openshift_cr.yaml) for an example of how to provide the required configuration for a MongoDB ReplicaSet.

See [here](../deploy/openshift/operator_openshift.yaml) for an example of how to configure the Operator deployment.

### Example

Consider the following example MongoDB resource definition:

```yaml
apiVersion: mongodb.com/v1
kind: MongoDB
metadata:
  name: example-mongodb
spec:
  members: 3
  type: ReplicaSet
  version: "4.0.6"
```
To upgrade this resource from `4.0.6` to `4.2.7`:

1. Edit the resource definition.

   a. Update `spec.version` to `4.2.7`.

   b. Update `spec.featureCompatibilityVersion` to `4.0`.

   ```yaml
   apiVersion: mongodb.com/v1
   kind: MongoDB
   metadata:
     name: example-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "4.2.7"
     featureCompatibilityVersion: "4.0"
   ```

   **NOTE:** Setting `featureCompatibilityVersion` to `4.0` disables [4.2 features incompatible with MongoDB 4.0](https://docs.mongodb.com/manual/release-notes/4.2-compatibility/#compatibility-enabled).

2. Reapply the configuration to Kubernetes:
   ```
   kubectl apply -f <example>.yaml --namespace <my-namespace>
   ```
