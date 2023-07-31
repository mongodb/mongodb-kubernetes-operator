# MongoDB Kubernetes Operator 0.8.1

## MongoDBCommunity Resource
- Changes
  - Connection string options
    - The MongoDBCommunity Resource now contains a new field ```additionalConnectionStringConfig``` where connection string options can be set, and they will apply to the connection string of every user.
    - Each user in the resource contains the same field ```additionalConnectionStringConfig``` and these options apply only for this user and will override any existing options in the resource.
    - The following options will be ignored `replicaSet`, `tls`, `ssl`, as they are set through other means.
    - [Sample](../config/samples/mongodb.com_v1_mongodbcommunity_additional_connection_string_options.yaml)
  - Support for Label and Annotations Wrapper
    - Additionally to the `specWrapper` for `statefulsets` we now support overriding `metadata.Labels` and `metadata.Annotations` via the `MetadataWrapper`.
    - [Sample](../config/samples/arbitrary_statefulset_configuration/mongodb.com_v1_metadata.yaml)

## Updated Image Tags

- mongodb-kubernetes-operator:0.8.1
- mongodb-agent:12.0.24.7719-1
- mongodb-kubernetes-readinessprobe:1.0.15

_All the images can be found in:_

https://quay.io/mongodb
https://hub.docker.com/r/mongodb/mongodb-community-server
