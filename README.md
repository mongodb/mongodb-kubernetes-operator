# MongoDB Community Kubernetes Operator #

<img align="left" src="https://mongodb-kubernetes-operator.s3.amazonaws.com/img/Leaf-Forest%402x.png">

This is a [Kubernetes Operator](https://coreos.com/operators/) which deploys MongoDB Community into Kubernetes clusters.

This codebase is currently _pre-alpha_, and is not ready for use.

If you are a MongoDB Enterprise customer, or need Enterprise features such as Backup, you can use the [MongoDB Enterprise Operator for Kubernetes](https://github.com/mongodb/mongodb-enterprise-kubernetes).

## Installation

### Prerequisites

Before you install the MongoDB Community Kubernetes Operator, you must:

1. Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
2. Have a Kubernetes solution available to use.
   If you need a Kubernetes solution, see the [Kubernetes documentation on picking the right solution](https://kubernetes.io/docs/setup). For testing, MongoDB recommends [Kind](https://kind.sigs.k8s.io/).
3. Clone this repository.
   ```
   git clone https://github.com/mongodb/mongodb-kubernetes-operator.git
   ```

### Installing the MongoDB Community Kubernetes Operator

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
      kubectl create -f deploy --namespace <my-namespace>
      ```
   b. Verify that the Operator installed successsfully:
      ```
      kubectl get pods --namespace <my-namespace>
      ```

## Usage

The `/deploy/crds` directory contains example MongoDB resources that you can modify and deploy.

### Deploying a MongoDB Resource

To deploy your first replica set:

1. Invoke the following `kubectl` command:
   ```
   kubectl apply -f deploy/crds/mongodb.com_v1_mongodb_cr.yaml --namespace <my-namespace>
   ```
2. Verify that the MongoDB resource deployed:
   ```
   kubectl get mongodb --namespace <my-namespace>
   ```

## Contributing

Please get familiar with the architecture.md document and then go ahead and read
the [contributing guide](contributing.md) guide.

Please file issues before filing PRs. For PRs to be accepted, contributors must sign our [CLA](https://www.mongodb.com/legal/contributor-agreement).

Reviewers, please ensure that the CLA has been signed by referring to [the contributors tool](https://contributors.corp.mongodb.com/) (internal link).

## License

The source code of this Operator is available under the Apache v2 license.

The MongoDB Agent binary in the agent/ directory may be used under the "Free for Commercial Use" license found in agent/LICENSE.
