# MongoDB Kubernetes Operator 0.8.3

## MongoDBCommunity Resource

- Changes
  - MongoDB 7.0.0 and onwards is not supported. Supporting it requires newer Automation Agent version that tentatively will be available in Q12024. The Operator will fail all deployments with this version. To ignore this error and force the Operator to reconcile these resources, use `IGNORE_MDB_7_ERROR` environment variable and set it to `true`. 