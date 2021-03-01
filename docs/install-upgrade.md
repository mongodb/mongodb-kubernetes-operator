# Install and Upgrade the Community Kubernetes Operator #

## Table of Contents

- [Install the Operator](#install-the-operator)
  - [Prerequisites](#prerequisites)
  - [Understand Deployment Scopes](#understand-deployment-scopes)
  - [Procedure](#procedure)
- [Upgrade the Operator](#upgrade-the-operator)

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
4. Review the possible Operator [deployment scopes](#understand-mongodb-community-operator-deployment-scopes) and configure the Operator to watch other namespaces, if necessary.

### Understand Deployment Scopes

You can deploy the MongoDB Community Kubernetes Operator with different scopes based on where you want to deploy MongoDB resources:

- [Operator in Same Namespace as Resources](#operator-in-same-namespace-as-resources)
- [Operator in Different Namespace Than Resources](#operator-in-different-namespace-than-resources)

#### Operator in Same Namespace as Resources

You scope the Operator to a namespace. The Operator watches MongoDB resources in that same [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/).

This is the default scope when you install the Operator using the [installation instructions](#procedure).

#### Operator in Different Namespace Than Resources

You scope the Operator to a namespace. The Operator watches MongoDB resources in other namespaces.

To configure the Operator to watch resources in other namespaces:

1. In the Operator [resource definition](../config/manager/manager.yaml), set the `WATCH_NAMESPACE` environment variable to one of the following values:

   - the namespace that you want the Operator to watch, or
   - `*` to configure the Operator to watch all namespaces in the cluster.

   ```yaml
       spec:
         containers:
           - name: mongodb-kubernetes-operator
             image: quay.io/mongodb/mongodb-kubernetes-operator:0.5.0
             command:
               - mongodb-kubernetes-operator
             imagePullPolicy: Always
             env:
               - name: WATCH_NAMESPACE
                 value: *
   ```

2. Run the following command to create cluster-wide roles and role-bindings in the default namespace:

   ```sh
   kubectl apply -f deploy/clusterwide
   ```
3. For each namespace that you want the Operator to watch, run the following commands to deploy a role and role-binding in that namespace:

   ```sh
   kubectl apply -f deploy/operator/role.yaml -n <namespace> && kubectl apply -f deploy/operator/role_binding.yaml -n <namespace>
   ```

4. [Install the operator](#procedure).

### Procedure

The MongoDB Community Kubernetes Operator is a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and a controller.

The Operator can be installed using the [Makefile](../Makefile) provided by `operator-sdk`

To install the MongoDB Community Kubernetes Operator:

1. Change to the directory in which you cloned the repository.
2. Install the [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

   a. Invoke the following command:
      ```
      kubectl apply -f config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml
      ```
   b. Verify that the Custom Resource Definitions installed successfully:
      ```
      kubectl get crd/mongodbcommunity.mongodbcommunity.mongodb.com
      ```
3. Install the necessary roles and role-bindings:

    a. Invoke the following command:
    ```
    kubectl apply -k config/rbac/
    ```
    b. Verify that the resources have been created:
    ```
    kubectl get role mongodb-kubernetes-operator

    kubectl get rolebinding mongodb-kubernetes-operator

    kubectl get serviceaccount mongodb-kubernetes-operator
    ```
4. Install the Operator.

   a. Invoke the following `kubectl` command to install the Operator in the specified namespace:
      ```
      kubectl create -f config/manager/manager.yaml --namespace <my-namespace>
      ```
   b. Verify that the Operator installed successsfully:
      ```
      kubectl get pods --namespace <my-namespace>
      ```

## Upgrade the Operator

To upgrade the MongoDB Community Kubernetes Operator to a specific version:

1. Change the version in the [operator yaml file](../config/manager/manager.yaml)
2. Change to the directory in which you cloned the repository.
3. Checkout the specific tag matching the operator version (e.g. `v0.5.1`)
4. Invoke the following `kubectl` command to upgrade the [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
   ```
   kubectl apply -f config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml
   ```
