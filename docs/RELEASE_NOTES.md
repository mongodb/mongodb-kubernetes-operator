# MongoDB Kubernetes Operator 0.10.0

## Released images signed

All container images published for the community operator are signed with our private key. This is visible on our Quay registry. Signature can be verified using our public key, which is available at [this address](https://cosign.mongodb.com/mongodb-enterprise-kubernetes-operator.pem).

## Logging changes
- The agent logging can be configured to stdout
- ReadinessProbe logging configuration can now be configured
- More can be found [here](logging.md).

## Overriding Mongod settings via the CRD 
- Example can be found [here](../config/samples/mongodb.com_v1_mongodbcommunity_override_ac_setting.yaml).

## ReadinessProbe error logging
- fixed a red herring which caused the probe to panic when the health status is not available. Instead it will just log the error

## Important Bumps
- Bumped K8S libs to 1.27
