# MongoDB Kubernetes Operator 0.8.0

## Kubernetes Operator

- Changes
  - The Operator uses now the official [MongoDB Community Server images](https://hub.docker.com/r/mongodb/mongodb-community-server).
    It is still possible to use the Docker Inc. images by altering the JSON configuration file:
    ```asd
          mongodb_image_name=mongo
          mongodb_image_repo_url=docker.io
    ```
    Alternatively, it is possible to the Operator environmental variables to:
    ```
          MONGODB_IMAGE=mongo
          MONGODB_REPO_URL=docker.io
    ```
    If the Operator is running using the default settings, the upgrade process will be automatic and seamless.


## Updated Image Tags


- mongodb-kubernetes-operator:0.8.0

_All the images can be found in:_

https://quay.io/mongodb
https://hub.docker.com/r/mongodb/mongodb-community-server
