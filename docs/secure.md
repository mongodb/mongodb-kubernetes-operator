# Secure MongoDB Resources #

## Table of Contents

- [Secure MongoDB Resource Connections using TLS](#secure-mongodb-resource-connections-using-tls)
- [Installing with Helm and cert-manager](#installing-with-helm-and-cert-manager)
  - [Prerequisites](#prerequisites)
  - [Procedure](#procedure)
- [Bring your own certificate](#bring-your-own-certificate)
  - [Prerequisites](#prerequisites-1)
  - [Procedure](#procedure-1)

## Secure MongoDB Resource Connections using TLS

You can configure the MongoDB Community Kubernetes Operator to use TLS certificates to encrypt traffic between:
- MongoDB hosts in a replica set, and
- Client applications and MongoDB deployments.

There are two methods currently supported:
- using [Helm](https://helm.sh/) and [cert-manager](https://cert-manager.io/): allows installing the MongoDB resource and configuring TLS in one step, using Helm, MongoDB Helm chart and an already installed cert-manager instance. With this setup, cert-manager is used to generate the Certificate Authority (CA) and the TLS certificate that will secure the MongoDB connections. Certificate rotation is then handled automatically by cert-manager.
- bring your own certificate: if you have your own Certificate Authority (CA), it can be used to generate and use a TLS certificate to secure MongoDB connections.

## Installing with Helm and cert-manager
### Prerequisites
Before you install MongoDB Helm chart with a set of values that will create the MongoDB resource with TLS configured, you must:
  - [install Helm](https://helm.sh/docs/intro/install/)
  - [install cert-manager](https://cert-manager.io/docs/installation/helm/#4-install-cert-manager)
### Procedure
1. Clone this repository:
   ```
   git clone https://github.com/mongodb/mongodb-kubernetes-operator.git
   ```
2. Optional: update _resource_ section from [Helm values](../helm-chart/values.yaml) with desired MongoDB resource configuration
3. Install MongoDB Helm chart:
   ```
   helm upgrade --install <helm deploy name> ./helm-chart \
    --namespace <mongodb namespace> --create-namespace \
    --set namespace=<mongodb namespace>,createResource=true,resource.tls.enabled=true,resource.tls.useCertManager=true
   ```

## Bring your own certificate
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
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
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
