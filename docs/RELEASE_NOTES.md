# MongoDB Kubernetes Operator 0.8.0

## MongoDBCommunity Resource
- Changes
  - Connection string options
    - The MongoDBCommunity Resource now contains a new field ```additionalConnectionStringConfig``` where connection string options can be set, and they will apply to the connection string of every user.
    - Each user in the resource contains the same field ```additionalConnectionStringConfig``` and these options apply only for this user and will override any existing options in the resource.
    - The following options will be ignored `replicaSet`, `tls`, `ssl`, as they are set through other means.
    - [Sample](../config/samples/mongodb.com_v1_mongodbcommunity_additional_connection_string_options.yaml)
  
## Kubernetes Operator

- Changes
  - The Operator now uses the official [MongoDB Community Server images](https://hub.docker.com/r/mongodb/mongodb-community-server).
    It is still possible to use the Docker Inc. images by altering the JSON configuration file:
    ```
          mongodb_image_name=mongo
          mongodb_image_repo_url=docker.io
    ```
    Alternatively, it is possible to the Operator environmental variables to:
    ```
          MONGODB_IMAGE=mongo
          MONGODB_REPO_URL=docker.io
    ```
    The upgrade process for the official MongoDB images is automatic when using the default settings provided by both,
    [kubectl](install-upgrade.md#install-the-operator-using-kubectl) and [Helm](install-upgrade.md#install-the-operator-using-helm)
    operator installation methods. Once the Operator boots up, it will replace `image` tags in the StatefulSets. If however,
    you're using customized deployments (by modifying `MONGODB_IMAGE` or `MONGODB_REPO_URL` environment variable in the Operator
    Deployment), please check if your settings are correct and if they are pointing to the right coordinates. The Operator
    still provides basic backwards compatibility with previous images (`docker.io/mongo`).

- `mongodb-readiness-hook` and `mongodb-version-upgrade-hook` images are now rebuilt daily, incorporating updates to system packages and security fixes. The binaries are built only once during the release process and used without changes in daily rebuilt


## Updated Image Tags

- mongodb-kubernetes-operator:0.8.0
- mongodb-agent:12.0.21.7698-1
- mongodb-kubernetes-readinessprobe:1.0.14
- mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.7

_All the images can be found in:_

https://quay.io/mongodb
https://hub.docker.com/r/mongodb/mongodb-community-server
