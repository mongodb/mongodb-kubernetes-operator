# MongoDB Community Kubernetes Operator #

<img align="right" src="https://mongodb-kubernetes-operator.s3.amazonaws.com/img/Leaf-Forest%402x.png">

This is a [Kubernetes Operator](https://coreos.com/operators/) which deploys MongoDB Community into Kubernetes clusters.

This codebase is currently _alpha_, and is not ready for production use.

If you are a MongoDB Enterprise customer, or need Enterprise features such as Backup, you can use the [MongoDB Enterprise Operator for Kubernetes](https://github.com/mongodb/mongodb-enterprise-kubernetes).

## Table of Contents

- [Install the Operator](#install-the-operator)
  - [Prerequisites](#prerequisites)
  - [Procedure](#procedure)
- [Upgrade the Operator](#upgrade-the-operator)
- [Deploy & Configure MongoDB Resources](#deploy-and-configure-a-mongodb-resource)
  - [Deploy a Replica Set](#deploy-a-replica-set)
  - [Upgrade MongoDB Version & FCV](#upgrade-your-mongodb-resource-version-and-feature-compatibility-version)
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

## Supported Features

The MongoDB Community Kubernetes Operator supports the following features:

- MongoDB Topology: [replica sets](https://docs.mongodb.com/manual/replication/)
- Upgrading and downgrading MongoDB server version
- Scaling replica sets up and down
- Reading from and writing to the replica set while scaling, upgrading, and downgrading. These operations are done in an "always up" manner.
- Reporting of MongoDB server state via the [MongoDB resource](/deploy/crds/mongodb.com_mongodb_crd.yaml) `status` field
- Use of any of the available [Docker MongoDB images](https://hub.docker.com/_/mongo/)
- Clients inside the Kubernetes cluster can connect to the replica set (no external connectivity)

### Planned Features
- TLS support for client/server communication
- Server internal authentication via keyfile
- Creating users with SCRAM-SHA authentication

## Contribute

Before you contribute to the MongoDB Community Kubernetes Operator, please read:

- [MongoDB Community Kubernetes Operator Architecture](architecture.md)
- [Contributing to MongoDB Community Kubernetes Operator](contributing.md)

Please file issues before filing PRs. For PRs to be accepted, contributors must sign our [CLA](https://www.mongodb.com/legal/contributor-agreement).

Reviewers, please ensure that the CLA has been signed by referring to [the contributors tool](https://contributors.corp.mongodb.com/) (internal link).

## License

The source code of this Operator is available under the Apache v2 license.

The MongoDB Agent binary in the agent/ directory may be used under the "Free for Commercial Use" license found in agent/LICENSE.
