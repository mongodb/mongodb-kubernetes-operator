# MongoDB Community Kubernetes Operator #

<img align="right" src="https://mongodb-kubernetes-operator.s3.amazonaws.com/img/Leaf-Forest%402x.png">

This is a [Kubernetes Operator](https://coreos.com/operators/) which deploys MongoDB Community into Kubernetes clusters.

If you are a MongoDB Enterprise customer, or need Enterprise features such as Backup, you can use the [MongoDB Enterprise Operator for Kubernetes](https://github.com/mongodb/mongodb-enterprise-kubernetes).

## Table of Contents

- [Install the Operator](#install-the-operator)
  - [Prerequisites](#prerequisites)
  - [Procedure](#procedure)
- [Upgrade the Operator](#upgrade-the-operator)
- [Deploy & Configure MongoDB Resources](#deploy-and-configure-a-mongodb-resource)
  - [Deploy a Replica Set](#deploy-a-replica-set)
  - [Upgrade MongoDB Version & FCV](#upgrade-your-mongodb-resource-version-and-feature-compatibility-version)
- [Secure MongoDB Resource Connections using TLS](#secure-mongodb-resource-connections-using-tls)
  - [Prerequisites](#prerequisites-1)
  - [Procedure](#procedure-1)
- [Supported Features](#supported-features)
- [Contribute](#contribute)
- [License](#license)

## Install the Operator

### Prerequisites

Before you install the MongoDB Community Kubernetes Operator, you must:

1. Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
2. Have a Kubernetes solution available to use.
   If you need a Kubernetes solution, see the [Kubernetes documentation on picking the right solution](https://kubernetes.io/docs/setup). For testing, MongoDB recommends [Kind](https://kind.sigs.k8s.io/).
3. Clone this repository.
   ```
   git clone https://github.com/mongodb/mongodb-kubernetes-operator.git
   ```

### Procedure

The MongoDB Community Kubernetes Operator is a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and a controller.

To install the MongoDB Community Kubernetes Operator:

1. Change to the directory in which you cloned the repository.
2. Install the [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

   a. Invoke the following `kubectl` command:
      ```
      kubectl create -f deploy/crds/mongodb.com_mongodb_crd.yaml
      ```
   b. Verify that the Custom Resource Definitions installed successfully:
      ```
      kubectl get crd/mongodb.mongodb.com
      ```
3. Install the Operator.

   a. Invoke the following `kubectl` command to install the Operator in the specified namespace:
      ```
      kubectl create -f deploy/ --namespace <my-namespace>
      ```
   b. Verify that the Operator installed successsfully:
      ```
      kubectl get pods --namespace <my-namespace>
      ```

## Upgrade the Operator

To upgrade the MongoDB Community Kubernetes Operator:

1. Change to the directory in which you cloned the repository.
2. Invoke the following `kubectl` command to upgrade the [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
   ```
   kubectl apply -f deploy/crds/mongodb.com_mongodb_crd.yaml
   ```

## Deploy and Configure a MongoDB Resource

The [`/deploy/crds`](deploy/crds) directory contains example MongoDB resources that you can modify and deploy.

### Deploy a Replica Set

To deploy your first replica set:

1. Invoke the following `kubectl` command:
   ```
   kubectl apply -f deploy/crds/mongodb.com_v1_mongodb_cr.yaml --namespace <my-namespace>
   ```
2. Verify that the MongoDB resource deployed:
   ```
   kubectl get mongodb --namespace <my-namespace>
   ```
3. connect clients to MongoDB replica set:
   ```
   mongodb://<metadata.name of the MongoDB resource>-svc.<namespace>.svc.cluster.local:27017/?replicaSet=<replica set name>
   ```

### Upgrade your MongoDB Resource Version and Feature Compatibility Version

You can upgrade the major, minor, and/or feature compatibility versions of your MongoDB resource. These settings are configured in your resource definition YAML file.

- To upgrade your resource's major and/or minor versions, set the `spec.version` setting to the desired MongoDB version.

- To modify your resource's [feature compatibility version](https://docs.mongodb.com/manual/reference/command/setFeatureCompatibilityVersion/), set the `spec.featureCompatibilityVersion` setting to the desired version.

If you update `spec.version` to a later version, consider setting `spec.featureCompatibilityVersion` to the current working MongoDB version to give yourself the option to downgrade if necessary. To learn more about feature compatibility, see [`setFeatureCompatibilityVersion`](https://docs.mongodb.com/manual/reference/command/setFeatureCompatibilityVersion/) in the MongoDB Manual.

#### Example

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

## Secure MongoDB Resource Connections using TLS

You can configure the MongoDB Community Kubernetes Operator to use TLS certificates to encrypt traffic between:

- MongoDB hosts in a replica set, and
- Client applications and MongoDB deployments.

### Prerequisites

Before you secure MongoDB resource connections using TLS, you must:

1. Create a PEM-encoded TLS certificate for the servers in the MongoDB resource using your own Certificate Authority (CA). The certificate must have one of the following:

   - A wildcard `Common Name` that matches the domain name of all of the replica set members:

     ```
     *.<metadata.name of the MongoDB resource>-svc.<namespace>.svc.cluster.local
     ```
   - The domain name for each of the replica set members as `Subject Alternative Names` (SAN):

     ```
     <metadata.name of the MongoDB resource>-0.<metadata.name of the MongoDB resource>-svc.<namespace>.svc.cluster.local
     <metadata.name of the MongoDB resource>-1.<metadata.name of the MongoDB resource>-svc.<namespace>.svc.cluster.local
     <metadata.name of the MongoDB resource>-2.<metadata.name of the MongoDB resource>-svc.<namespace>.svc.cluster.local
     ```

1. Create a Kubernetes ConfigMap that contains the certificate for the CA that signed your server certificate. The key in the ConfigMap that references the certificate must be named `ca.crt`. Kubernetes configures this automatically if the certificate file is named `ca.crt`:
   ```
   kubectl create configmap <tls-ca-configmap-name> --from-file=ca.crt --namespace <namespace>
   ```

   For a certificate file with any other name, you must define the `ca.crt` key manually:
   ```
   kubectl create configmap <tls-ca-configmap-name> --from-file=ca.crt=<certificate-file-name>.crt --namespace <namespace>
   ```

1. Create a Kubernetes secret that contains the server certificate and key for the members of your replica set. For a server certificate named `server.crt` and key named `server.key`:
   ```
   kubectl create secret tls <tls-secret-name> --cert=server.crt --key=server.key --namespace <namespace>
   ```

### Procedure

To secure connections to MongoDB resources using TLS:

1. Add the following fields to the MongoDB resource definition:

   - `spec.security.tls.enabled`: Encrypts communications using TLS certificates between MongoDB hosts in a replica set and client applications and MongoDB deployments. Set to `true`.
   - `spec.security.tls.optional`: (**Optional**) Enables the members of the replica set to accept both TLS and non-TLS client connections. Equivalent to setting the MongoDB[`net.tls.mode`](https://docs.mongodb.com/manual/reference/configuration-options/#net.tls.mode) setting to `preferSSL`. If omitted, defaults to `false`.

     ---
     **NOTE**

     When you enable TLS on an existing replica set deployment:

     a. Set `spec.security.tls.optional` to `true`.

     b. Apply the configuration to Kubernetes.

     c. Upgrade your existing clients to use TLS.

     d. Remove the `spec.security.tls.optional` field.

     e. Complete the remaining steps in the procedure.

     ---
   - `spec.security.tls.certificateKeySecretRef.name`: Name of the Kubernetes secret that contains the server certificate and key that you created in the [prerequisites](#prerequisites-1).
   - `spec.security.tls.caConfigMapRef.name`: Name of the Kubernetes ConfigMap that contains the Certificate Authority certificate used to sign the server certificate that you created in the [prerequisites](#prerequisites-1).

   ```yaml
   apiVersion: mongodb.com/v1
   kind: MongoDB
   metadata:
     name: example-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "4.2.7"
     security:
       tls:
         enabled: true
         certificateKeySecretRef:
           name: <tls-secret-name>
         caConfigMapRef:
           name: <tls-ca-configmap-name>
   ```

1. Apply the configuration to Kubernetes:
   ```
   kubectl apply -f <example>.yaml --namespace <my-namespace>
   ```
1. From within the Kubernetes cluster, connect to the MongoDB resource.
   - If `spec.security.tls.optional` is omitted or `false`: clients must
     establish TLS connections to the MongoDB servers in the replica set.
   - If `spec.security.tls.optional` is true, clients can establish TLS or
     non-TLS connections to the MongoDB servers in the replica set.

   See the documentation for your connection method to learn how to establish a TLS connection to a MongoDB server.

## Supported Features

The MongoDB Community Kubernetes Operator supports the following features:

- MongoDB Topology: [replica sets](https://docs.mongodb.com/manual/replication/)
- Upgrading and downgrading MongoDB server version
- Scaling replica sets up and down
- Reading from and writing to the replica set while scaling, upgrading, and downgrading. These operations are done in an "always up" manner.
- Reporting of MongoDB server state via the [MongoDB resource](/deploy/crds/mongodb.com_mongodb_crd.yaml) `status` field
- Use of any of the available [Docker MongoDB images](https://hub.docker.com/_/mongo/)
- Clients inside the Kubernetes cluster can connect to the replica set (no external connectivity)
- TLS support for client-to-server and server-to-server communication

### Planned Features
- Server internal authentication via keyfile
- Creating users with SCRAM-SHA authentication

## Contribute

Before you contribute to the MongoDB Community Kubernetes Operator, please read:

- [MongoDB Community Kubernetes Operator Architecture](architecture.md)
- [Contributing to MongoDB Community Kubernetes Operator](contributing.md)

Please file issues before filing PRs. For PRs to be accepted, contributors must sign our [CLA](https://www.mongodb.com/legal/contributor-agreement).

Reviewers, please ensure that the CLA has been signed by referring to [the contributors tool](https://contributors.corp.mongodb.com/) (internal link).

## License

Please see the [LICENSE](LICENSE) file.
