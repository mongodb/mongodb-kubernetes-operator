# Enable X.509 Authentication

You can use Helm or `kubectl` to enable X.509 authentication for the 
MongoDB Agent and client.

## Prerequisites

1. Add the `cert-manager` repository to your `helm` repository list and
   ensure it's up to date:

   ```
   helm repo add jetstack https://charts.jetstack.io
   helm repo update
   ```

1. Install `cert-manager`:

   ```
   helm install cert-manager jetstack/cert-manager --namespace cert-manager \ 
   --create-namespace --set installCRDs=true
   ```

## Use Helm to Enable X.509 Authentication

You can use Helm to install and deploy the MongoDB Community Kubernetes 
Operator with X.509 Authentication enabled for the MongoDB Agent and 
client. To learn more, see [Install the Operator using Helm](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/docs/install-upgrade.md#install-the-operator-using-helm).

1. To deploy the MongoDB Community Kubernetes Operator, copy and paste 
   the following command and replace the `<namespace>` variable with the 
   namespace:

   **Note:**

   The following command deploys a sample resource with X.509 enabled
   for both the MongoDB Agent and client authentication. It also creates
   a sample X.509 user and the certificate that the user can use to 
   authenticate.

   ```
   helm upgrade --install community-operator mongodb/community-operator \
   --namespace <namespace> --set namespace=<namespace> --create-namespace \
   --set resource.tls.useCertManager=true --set resource.tls.enabled=true \
   --set resource.tls.useX509=true --set resource.tls.sampleX509User=true \
   --set createResource=true
   ```

## Use `kubectl` to Enable X.509 Authentication

You can use Helm to install and deploy the MongoDB Community Kubernetes 
Operator with X.509 Authentication enabled for the MongoDB Agent and 
client.

1. To install the MongoDB Community Kubernetes Operator, see 
   [Install the Operator using kubectl](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/docs/install-upgrade.md#install-the-operator-using-kubectl).

1. To create a CA, ConfigMap, secrets, issuer, and certificate, see 
   [Enable External Access to a MongoDB Deployment](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/docs/external_access.md).

1. Create a YAML file for the  MongoDB Agent certificate. For an example, 
   see [agent-certificate.yaml](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/config/samples/external_access/agent-certificate.yaml).

   **Note:**

   - For the `spec.issuerRef.name` parameter, specify the 
     `cert-manager` issuer that you created previously.
   - For the `spec.secretName` parameter, specify the same 
     value as the `spec.security.authentication.agentCertificateSecretRef` 
     parameter in your resource. This secret should contain a signed 
     X.509 certificate and a private key for the MongoDB agent.

1. To apply the file, copy and paste the following command and replace 
   the `<agent-certificate>` variable with the name of your MongoDB Agent 
   certificate and the `<namespace>` variable with the namespace:

   ```
   kubectl apply -f <agent-certificate>.yaml --namespace <namespace>
   ```

1. Create a YAML file for your resource. For an example, see 
   [mongodb.com_v1_mongodbcommunity_x509.yaml](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/config/samples/mongodb.com_v1_mongodbcommunity_x509.yaml).

   **Note:**

   - For the `spec.security.tls.certificateKeySecretRef.name` parameter,
     specify a reference to the secret that contains the private key and
     certificate to use for TLS. The operator expects the PEM encoded key 
     and certificate available at "tls.key" and "tls.crt". Use the same 
     format used for the standard "kubernetes.io/tls" Secret type, but no 
     specific type is required. Alternatively, you can provide 
     an entry called "tls.pem" that contains the concatenation of the 
     certificate and key. If all of "tls.pem", "tls.crt" and "tls.key" 
     are present, the "tls.pem" entry needs to equal the concatenation 
     of "tls.crt" and "tls.key".

   - For the `spec.security.tls.caConfigMapRef.name` parameter, specify
     the ConfigMap that you created previously.

   - For the `spec.authentication.modes` parameter, specify `X509`.
   
   - If you have multiple authentication modes, specify the 
     `spec.authentication.agentMode` parameter.

   - The `spec.authentication.agentCertificateSecretRef` parameter
     defaults to `agent-certs`.

   - For the `spec.users.db` parameter, specify `$external`.

   - Do not set the `spec.users.scramCredentialsSecretName` parameter 
     and the `spec.users.passwordSecretRef` parameters.

1. To apply the file, copy and paste the following command and replace 
   the `<replica-set>` variable with your resource and the `<namespace>`
   variable with the namespace:

   ```
   kubectl apply -f <replica-set>.yaml --namespace <namespace>
   ```

1. Create a YAML file for the client certificate. For an example, see 
   [cert-x509.yaml](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/config/samples/external_access/cert-x509.yaml).

1. To apply the file, copy and paste the following command and replace 
   the `<client-certificate>` variable with the name of your client 
   certificate and the `<namespace>` variable with the namespace:

   ```
   kubectl apply -f <client-certificate>.yaml --namespace <namespace>
   ```
