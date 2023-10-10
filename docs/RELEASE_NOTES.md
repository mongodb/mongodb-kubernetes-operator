# MongoDB Kubernetes Operator 0.8.3

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
  - Add support for configuring [logRotate](https://www.mongodb.com/docs/ops-manager/current/reference/cluster-configuration/#mongodb-instances) on the automation-agent. The settings can be found under `processes[n].logRotate.<setting>`.
  - Additionally, [systemLog](https://www.mongodb.com/docs/manual/reference/configuration-options/#systemlog-options) can now be configured. In particular the settings: `path`, `destination` and `logAppend`.
  - MongoDB 7.0.0 and onwards is not supported. Supporting it requires a newer Automation Agent version. Until a new version is available, the Operator will fail all deployments with this version. To ignore this error and force the Operator to reconcile these resources, use `IGNORE_MDB_7_ERROR` environment variable and set it to `true`.
  - Introduced support for ARM64 architecture
    - A manifest supporting both AMD64 and ARCH64 architectures is released for each version.
  - `ubuntu` based images are deprecated, users should move to `ubi` images next release.