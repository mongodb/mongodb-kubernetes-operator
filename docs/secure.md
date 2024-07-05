# Secure MongoDBCommunity Resources #

## Table of Contents

- [Secure MongoDBCommunity Resource Connections using TLS](#secure-mongodbcommunity-resource-connections-using-tls)
  - [Prerequisites](#prerequisites)
  - [Procedure](#procedure)

## Secure MongoDBCommunity Resource Connections using TLS

You can configure the MongoDB Community Kubernetes Operator to use TLS 
certificates to encrypt traffic between:

- MongoDB hosts in a replica set, and
- Client applications and MongoDB deployments.

The Operator automates TLS configuration through its integration with 
[cert-manager](https://cert-manager.io/), a certificate management tool for 
Kubernetes.

### Prerequisites

Before you secure MongoDBCommunity resource connections using TLS, you 
must [Create a database user](../docs/users.md) to authenticate to your 
MongoDBCommunity resource.

### Procedure

To secure connections to MongoDBCommunity resources with TLS using `cert-manager`:

1. Add the `cert-manager` repository to your `helm` repository list and
   ensure it's up to date:

   ```
   helm repo add jetstack https://charts.jetstack.io
   helm repo update
   ```

2. Install `cert-manager`:

   ```
   helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true
   ```

3. Create a TLS-secured MongoDBCommunity resource:

    This assumes you already have the operator installed in namespace `<namespace>`

   ```
   helm upgrade --install community-operator mongodb/community-operator \
   --namespace <namespace> --set resource.tls.useCertManager=true \
   --set createResource=true --set resource.tls.enabled=true \
   --set namespace=<namespace>
   ```

  This creates a resource secured with TLS and generates the necessary
  certificates with `cert-manager` according to the values specified in
  the `values.yaml` file in the Community Kubernetes Operator 
  [chart repository](https://github.com/mongodb/helm-charts/tree/main/charts/community-operator).

  `cert-manager` automatically reissues certificates according to the
  value of `resource.tls.certManager.renewCertBefore`. To alter the 
  reissuance interval, either: 
  
  - Set `resource.tls.certManager.renewCertBefore` in `values.yaml` to 
     the desired interval in hours before running `helm upgrade`

  - Set `spec.renewBefore` in the Certificate resource file generated
     by `cert-manager` to the desired interval in hours after running 
     `helm upgrade`
  


1. Test your connection over TLS by 

   - Connecting to a `mongod` container inside a pod using `kubectl`:

   ```
   kubectl exec -it <mongodb-replica-set-pod> -c mongod -- bash
   ```

   Where `mongodb-replica-set-pod` is the name of a pod from your MongoDBCommunity resource

   - Then, use `mongosh` to connect over TLS:
   For how to get the connection string look at [Deploy A Replica Set](deploy-configure.md#deploy-a-replica-set)

   ```
   mongosh "<connection-string>" --tls --tlsCAFile /var/lib/tls/ca/ca.crt --tlsCertificateKeyFile /var/lib/tls/server/*.pem 
   ```

   Where `mongodb-replica-set` is the name of your MongoDBCommunity 
   resource, `namespace` is the namespace of your deployment
   and  `connection-string` is a connection string for your `<mongodb-replica-set>-svc` service.