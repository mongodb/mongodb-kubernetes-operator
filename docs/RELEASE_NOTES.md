# MongoDB Kubernetes Operator 0.X.X

## MongoDBCommunity Resource

- Changes
  - Introduced support for X.509 authentication for client and agent
    - `spec.security.authentication.modes` now supports value `X509`
    - The agent authentication mode will default to the value in `spec.security.authentication.modes` if there is only one specified. 
    - Otherwise, agent authentication will need to be specified through `spec.security.authentication.agentMode`.
    - When agent authentication is set to `X509`, the field `spec.security.authentication.agentCertificateSecretRef` can be set (default is `agent-certs`).
    - The secret that `agentCertificateSecretRef` points to should contain a signed X.509 certificate (under the `tls.crt` key) and a private key (under `tls.key`) for the agent.
    - X.509 users can be added the same way as before under `spec.users`. The `db` field must be set to `$external` for X.509 authentication.
    - For these users, `scramCredentialsSecretName` and `passwordSecretRef` should **not** be set.
    - Sample resource [yaml](config/samples/mongodb.com_v1_mongodbcommunity_x509.yaml)
    - Sample agent certificate [yaml](config/samples/external_access/agent-certificate.yaml)

# MongoDB Kubernetes Operator 0.8.2

## Kubernetes Operator

- Changes
  - Fix a bug when overriding tolerations causing an endless reconciliation loop ([1344](https://github.com/mongodb/mongodb-kubernetes-operator/issues/1344)).

## Updated Image Tags

- mongodb-kubernetes-operator:0.8.2

_All the images can be found in:_

https://quay.io/mongodb
https://hub.docker.com/r/mongodb/mongodb-community-server
