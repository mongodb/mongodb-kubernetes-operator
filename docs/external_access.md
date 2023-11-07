## Enable External Access to a MongoDB Deployment

This guide assumes that the operator is installed and a MongoDB deployment is yet to be done but you have a chosen namespace that you are installing into. We will install cert-manager and then generate certificates and configure split-horizon to support internal and external DNS names for configuring external access to the replicaset.

### Install cert-manager

```sh
kubectl create namespace cert-manager
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --version v1.3.1 \
  --set installCRDs=true
```

### Install mkcert and generate CA

```sh
brew install mkcert # for Mac
#for Linux / Windows systems look at https://github.com/FiloSottile/mkcert
mkcert -install
```

Execute ```mkcert --CAROOT``` to note the location of the generated root CA key and cert.

### Retrieve the CA and create configmaps and secrets

Use the files that you found in the previous step. Replace ```<your-namespace>``` with your chosen namespace

```sh
kubectl create configmap ca-config-map --from-file=ca.crt=<path-to-ca.crt> --namespace <your-namespace>

kubectl create secret tls ca-key-pair  --cert=<path-to-ca.crt>  --key=<path-to-ca.key> --namespace <your-namespace>
```

### Create the Cert Manager issuer and secret

Edit the file [cert-manager-certificate.yaml](../config/samples/external_access/cert-manager-certificate.yaml) to replace ```<mongodb-name>``` with your MongoDB deployment name. Also replace ```<domain-rs-1>```, ```<domain-rs-2>```, and ```<domain-rs-3>``` with the external FQDNs of the MongoDB replicaset members. Please remember that you will have to add an equal number of entries for each member of the replicaset, for example:

```yaml
...
spec:
  members: 3
  type: ReplicaSet
  replicaSetHorizons:
  - horizon1: <domain1-rs-1>:31181
    horizon2: <domain2-rs-1>:31181
  - horizon1: <domain1-rs-2>:31182
    horizon2: <domain2-rs-2>:31182
  - horizon1: <domain1-rs-3>:31183
    horizon2: <domain2-rs-3>:31183
...
```

Apply the manifests. Replace ```<your-namespace>``` with the namespace you are using for the deployment.

```sh
kubectl apply -f config/samples/external_access/cert-manager-issuer.yaml --namespace <your-namespace>
kubectl apply -f config/samples/external_access/cert-manager-certificate.yaml --namespace <your-namespace>
```

### Create the MongoDB deployment

Edit [mongodb.com_v1_mongodbcommunity_cr.yaml](../config/samples/external_access/mongodb.com_v1_mongodbcommunity_cr.yaml). Replace <mongodb-name> with the desired MongoDB deployment name -- this should be the same as in the previous step. Replace ```<domain-rs-1>```, ```<domain-rs-2>```, and ```<domain-rs-3>``` with the external FQDNs of the MongoDB replicaset members. Please remember that you should have the same number of entries in this section as the number of your replicaset members. You can also edit the ports for external access to your preferred numbers in this section -- you will have to remember to change them in the next step too. Change ```<your-admin-password>``` to your desired admin password for MongoDB.

Apply the manifest.

```sh
kubectl apply -f config/samples/external_access/mongodb.com_v1_mongodbcommunity_cr.yaml --namespace <your-namespace>
```

Wait for the replicaset to be available.

### Create the external NodePort services for accessing the MongoDB deployment from outside the Kubernetes cluster

Edit [external_services.yaml](../config/samples/external_access/external_services.yaml) and replace ```<mongodb-name>``` with the MongoDB deployment name that you have used in the preceeding steps. You can change the ```nodePort``` and ```port``` to reflect the changes (if any) you have made in the previous steps.

Apply the manifest.

```sh
kubectl apply -f config/samples/external_access/external_services.yaml --namespace <your-namespace>
```

### Retrieve the certificates from a MongoDB replicaset member

```sh
kubectl exec --namespace <your-namespace>  -it <mongodb-name>-0 -c mongod -- bash
```

Once inside the container ```cat``` and copy the contents of the ```.pem``` file in ```/var/lib/tls/server``` into a file on your local system.

### Connect to the MongoDB deployment from outside the Kubernetes cluster

This is an example to connect to the MongoDB cluster with Mongo shell. Use the CA from ```mkcert``` and the certificate from the previous step. Replace the values in the command from the preceeding steps.

```sh
mongosh --tls --tlsCAFile ca.crt --tlsCertificateKeyFile key.pem --username my-user --password <your-admin-password> mongodb://<domain-rs-1>:31181,<domain-rs-2>:31182,<domain-rs-3>:31183
```

### Conclusion
At this point, you should be able to connect to the MongoDB deployment from outside the cluster. Make sure that you can resolve to the FQDNs for the replicaset members where you have the Mongo client installed.
