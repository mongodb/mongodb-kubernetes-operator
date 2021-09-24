# Install and Upgrade the Community Kubernetes Operator #

## Table of Contents

- [Install the Operator](#install-the-operator)
  - [Prerequisites](#prerequisites)
  - [Understand Deployment Scopes](#understand-deployment-scopes)
  - [Configure the MongoDB Docker Image or Container Registry](#configure-the-mongodb-docker-image-or-container-registry)
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
4. **Optional** Review the possible Operator [deployment scopes](#understand-deployment-scopes) and configure the Operator to watch other namespaces.
5. **Optional** Configure the [MongoDB Docker image or container registry](#configure-the-mongodb-docker-image-or-container-registry).

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
                 value: "*"
   ```

2. Modify the [clusterRoleBinding](../deploy/clusterwide/role_binding.yaml) namespace value for the serviceAccount `mongodb-kubernetes-operator` to the namespace in which the operator is deployed.

3. Run the following command to create cluster-wide roles and role-bindings in the default namespace:

   ```sh
   kubectl apply -f deploy/clusterwide
   ```
4. For each namespace that you want the Operator to watch, run the following
   commands to deploy a Role, RoleBinding and ServiceAccount in that namespace:

   ```sh
   kubectl apply -k config/rbac --namespace <my-namespace>
   ```

5. [Install the operator](#procedure).

### Configure the MongoDB Docker Image or Container Registry

By default, the Operator pulls the MongoDB database Docker image from `registry.hub.docker.com/library/mongo`.

To configure the Operator to use a different image or container registry
for MongoDB Docker images:

1. In the Operator [resource definition](../config/manager/manager.yaml), set the `MONGODB_IMAGE` and `MONGODB_REPO_URL` environment variables:

   | Environment Variable | Description | Default |
   |----|----|----|
   | `MONGODB_IMAGE` | From the `MONGODB_REPO_URL`, absolute path to the MongoDB Docker image that you want to deploy. | `"mongo"` |
   | `MONGODB_REPO_URL` | URL of the container registry that contains the MongoDB Docker image that you want to deploy. | `"docker.io"` |

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

3. [Install the operator](#procedure).

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
      kubectl delete crd mongodb.mongodb.com
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
