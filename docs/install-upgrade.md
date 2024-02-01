# Install and Upgrade the Community Kubernetes Operator #

## Table of Contents

- [Prerequisites](#prerequisites)
- [Install the Operator](#install-the-operator)
  - [Understand Deployment Scopes](#understand-deployment-scopes)
    - [Operator in Same Namespace as Resources](#operator-in-same-namespace-as-resources)
    - [Operator in Different Namespace Than Resources](#operator-in-different-namespace-than-resources)
  - [Install the Operator using Helm](#install-the-operator-using-Helm)
     - [Prerequisites to Install using Helm](#prerequisites-to-install-using-Helm)
     - [Procedure using Helm](#procedure-using-Helm)
  - [Install the Operator using kubectl](#install-the-operator-using-kubectl)
     - [Prerequisites to Install using kubectl](#prerequisites-to-install-using-kubectl)
     - [Install in a Different Namespace using kubectl](#install-in-a-different-namespace-using-kubectl)
     - [Configure the MongoDB Docker Image or Container Registry](#configure-the-mongodb-docker-image-or-container-registry)
     - [Procedure using kubectl](#procedure-using-kubectl)
- [Upgrade the Operator](#upgrade-the-operator)
- [Rotating TLS certificate for the MongoDB deployment](#rotating-tls-certificate-for-the-mongodb-deployment)

## Prerequisites

- A Kubernetes cluster with nodes with x86-64/AMD64 processors (either all, or a separate node pool)

## Install the Operator

The MongoDB Community Kubernetes Operator is a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and a controller.

Use the following resources to prepare your implementation and install the Community Operator:

- [Understand Deployment Scopes](#understand-deployment-scopes)
- [Install the Operator using Helm](#install-the-operator-using-Helm)
- [Install the Operator using kubectl](#install-the-operator-using-kubectl)

### Understand Deployment Scopes

You can deploy the MongoDB Community Kubernetes Operator with different scopes based on where you want to deploy MongoDBCommunity resources:

- [Operator in Same Namespace as Resources](#operator-in-same-namespace-as-resources)
- [Operator in Different Namespace Than Resources](#operator-in-different-namespace-than-resources)

#### Operator in Same Namespace as Resources

You scope the Operator to a namespace. The Operator watches MongoDBCommunity resources in that same [namespace](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/).

This is the default scope when you [install the Operator using Helm](#install-the-operator-using-helm) or [install the Operator using kubectl](#install-the-operator-using-kubectl).

#### Operator in Different Namespace Than Resources

You scope the Operator to a namespace. The Operator watches MongoDBCommunity resources in other namespaces.

To deploy the Operator in a different namespace than the resources, [Install in a Different Namespace using Helm](#install-in-a-different-namespace-using-helm) or [Install in a Different Namespace using kubectl](#install-in-a-different-namespace-using-kubectl).

### Install the Operator using Helm

You can install the Operator using the [MongoDB Helm Charts](https://mongodb.github.io/helm-charts/).

#### Prerequisites to Install using Helm

Before you install the MongoDB Community Kubernetes Operator using Helm, you must:

1. Have a Kubernetes solution available to use.
   If you need a Kubernetes solution, see the [Kubernetes documentation on picking the right solution](https://kubernetes.io/docs/setup). For testing, MongoDB recommends [Kind](https://kind.sigs.k8s.io/).
2. [Install Helm](https://helm.sh/docs/intro/install/).
3. Add the [MongoDB Helm Charts for Kubernetes](https://mongodb.github.io/helm-charts/) repository to Helm by running the following command:
   ```
   helm repo add mongodb https://mongodb.github.io/helm-charts
   ```

#### Procedure using Helm

Use one of the following procedures to install the Operator using Helm:

- [Install in the Default Namespace using Helm](#install-in-the-default-namespace-using-helm)
- [Install in a Different Namespace using Helm](#install-in-a-different-namespace-using-helm)

##### Install in the Default Namespace using Helm

To install the Custom Resource Definitions and the Community Operator in
the `default` namespace using Helm, run the install command from the
terminal:
   ```
   helm install community-operator mongodb/community-operator
   ```

If you already installed the `community-operator-crds` Helm chart, you must
include `--set community-operator-crds.enabled=false` when installing the Operator:
   ```
   helm install community-operator mongodb/community-operator --set community-operator-crds.enabled=false
   ```

##### Install in a Different Namespace using Helm

To install the Custom Resource Definitions and the Community Operator in
a different namespace using Helm, run the install
command with the `--namespace` flag from the terminal. Include the `--create-namespace`
flag if you are creating a new namespace.
   ```
   helm install community-operator mongodb/community-operator --namespace mongodb [--create-namespace]
   ```

To configure the Operator to watch resources in another namespace, run the following command from the terminal. Replace `example` with the namespace the Operator should watch:

   ```
   helm install community-operator mongodb/community-operator --set operator.watchNamespace="example"
   ```

### Install the Operator using kubectl

You can install the Operator using `kubectl` instead of Helm.

#### Prerequisites to Install using kubectl

Before you install the MongoDB Community Kubernetes Operator using `kubectl`, you must:

1. Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).
2. Have a Kubernetes solution available to use.
   If you need a Kubernetes solution, see the [Kubernetes documentation on picking the right solution](https://kubernetes.io/docs/setup). For testing, MongoDB recommends [Kind](https://kind.sigs.k8s.io/).
3. Clone this repository.
   ```
   git clone https://github.com/mongodb/mongodb-kubernetes-operator.git
   ```
4. **Optional** Configure the Operator to watch other namespaces.
5. **Optional** Configure the [MongoDB Docker image or container registry](#configure-the-mongodb-docker-image-or-container-registry).

##### Install in a Different Namespace using kubectl

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
                 value: "*"
   ```

2. Modify the [clusterRoleBinding](../deploy/clusterwide/cluster_role_binding.yaml) namespace value for the serviceAccount `mongodb-kubernetes-operator` to the namespace in which the operator is deployed.

3. Run the following command to create cluster-wide roles and role-bindings in the default namespace:

   ```sh
   kubectl apply -f deploy/clusterwide
   ```
4. For each namespace that you want the Operator to watch, run the following
   commands to deploy a Role, RoleBinding and ServiceAccount in that namespace:

   ```sh
   kubectl apply -k config/rbac --namespace <my-namespace>
   ```

   *Note: If you need the operator to have permission over multiple namespaces, for ex: when configuring the operator to have the `connectionStringSecret` in a different `namespace`, make sure
   to apply the `RBAC` in all the relevant namespaces.*


5. [Install the operator](#procedure-using-kubectl).

##### Configure the MongoDB Docker Image or Container Registry

By default, the Operator pulls the MongoDB database Docker image from `registry.hub.docker.com/library/mongo`.

To configure the Operator to use a different image or container registry
for MongoDB Docker images:

1. In the Operator [resource definition](../config/manager/manager.yaml), set the `MONGODB_IMAGE` and `MONGODB_REPO_URL` environment variables:

   **NOTE:** Use the official
   [MongoDB Community Server images](https://hub.docker.com/r/mongodb/mongodb-community-server).
   Official images provide the following advantages:

   - They are rebuilt daily for the latest upstream
     vulnerability fixes.
   - MongoDB tests, maintains, and supports them.

   | Environment Variable | Description | Default                      |
   |----|------------------------------|------------------------------|
   | `MONGODB_IMAGE` | From the `MONGODB_REPO_URL`, absolute path to the MongoDB Docker image that you want to deploy. | `"mongodb-community-server"` |
   | `MONGODB_REPO_URL` | URL of the container registry that contains the MongoDB Docker image that you want to deploy. | `"quay.io/mongodb"`          |

   ```yaml
       spec:
         containers:
           - name: mongodb-kubernetes-operator
             image: quay.io/mongodb/mongodb-kubernetes-operator:0.5.1
             command:
               - mongodb-kubernetes-operator
             imagePullPolicy: Always
             env:
               - name: MONGODB_IMAGE
                 value: <path/to/image>
               - name: MONGODB_REPO_URL
                 value: <container-registry-url>
   ```

2. Save the file.

3. [Install the operator](#procedure-using-kubectl).

#### Procedure using kubectl

The Operator can be installed using `kubectl` and the [Makefile](../Makefile).

To install the MongoDB Community Kubernetes Operator using kubectl:

1. Change to the Community Operator's directory.
2. Install the [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

   a. Invoke the following command:
      *Make sure to apply the CRD file from the [git tag version](https://github.com/mongodb/mongodb-kubernetes-operator/tags) of the operator you are attempting to install*.
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
    kubectl apply -k config/rbac/ --namespace <my-namespace>
    ```
    b. Verify that the resources have been created:
    ```
    kubectl get role mongodb-kubernetes-operator --namespace <my-namespace>

    kubectl get rolebinding mongodb-kubernetes-operator --namespace <my-namespace>

    kubectl get serviceaccount mongodb-kubernetes-operator --namespace <my-namespace>
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

The release v0.6.0 had some breaking changes (see https://github.com/mongodb/mongodb-kubernetes-operator/releases/tag/v0.6.0) requiring special steps to upgrade from a pre-0.6.0 Version.
As always, have backups.
Make sure you run commands in the correct namespace.

1. Prepare for the upgrade.

   a. Migrate your cr by updating apiVersion and kind to
      ```
      apiVersion: mongodbcommunity.mongodb.com/v1
      kind: MongoDBCommunity
      ```
      If you upgrade from pre-0.3.0 you need to also add the field spec.users[n].scramCredentialsSecretName for each resource. This will be used to determine the name of the generated secret which stores MongoDB user credentials. This field must comply with DNS-1123 rules (see https://kubernetes.io/docs/concepts/overview/working-with-objects/names/).
   b. Plan a downtime.
2. Out with the old

   a. Delete the old operator.
      ```
      kubectl delete deployments.apps mongodb-kubernetes-operator
      ```
   b. Delete the old statefulset.
      ```
      kubectl delete statefulsets.apps mongodb
      ```
   c. Delete the old customResourceDefinition. Not strictly needed but no need to keep it around anymore (unless you got more installations of operator in your cluster)
      ```
      kubectl delete crd mongodbcommunity.mongodbcommunity.mongodb.com
      ```
3. In with the new

   Follow the normal installation procedure above.
4. Start up your Replica Set again
   a. Re-create your cr using the new Version from Step 1.a
   b. Patch your statefulset to have it update the permissions
      ```
      kubectl patch statefulset <sts-name> --type='json' --patch '[ {"op":"add","path":"/spec/template/spec/initContainers/-", "value": { "name": "change-data-dir-permissions", "image": "busybox", "command": [ "chown", "-R", "2000", "/data" ], "securityContext": { "runAsNonRoot": false, "runAsUser": 0, "runAsGroup":0 }, "volumeMounts": [ { "mountPath": "/data", "name" : "data-volume" } ] } } ]'
      ```
   c. Delete your pod manually
      Since you added your cr in step a. kubernetes will immediately try to get your cluster up and running.
      You will now have one pod that isn't working since it got created before you patched your statefulset with the additional migration container.
      Delete that pod.
      ```
      kubectl delete pod <sts-name>-0
      ```
   d. You're done. Now Kubernetes will create the pod fresh, causing the migration to run and then the pod to start up. Then kubernetes will proceed creating the next pod until it reaches the number specified in your cr.

## Rotating TLS certificate for the MongoDB deployment

Renew the secret for your TLS certificates
```
kubectl create secret tls <secret_name> \
  --cert=<replica-set-tls-cert> \
  --key=<replica-set-tls-key> \
  --dry-run=client \
   -o yaml |
kubectl apply -f -
```
*`secret_name` is what you've specified under `Spec.Security.TLS.CertificateKeySecret.Name`*.

If you're using a tool like cert-manager, you can follow [these instructions](https://cert-manager.io/docs/usage/certificate/#renewal) to rotate the certificate.
The operator should would watch the secret change and re-trigger a reconcile process.
