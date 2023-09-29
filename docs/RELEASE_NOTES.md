# MongoDB Kubernetes Operator 0.8.3

## MongoDBCommunity Resource

- Changes
  - MongoDB 7.0.0 and onwards is not supported. Supporting it requires a newer Automation Agent version. Until a new version is available, the Operator will fail all deployments with this version. To ignore this error and force the Operator to reconcile these resources, use `IGNORE_MDB_7_ERROR` environment variable and set it to `true`. 