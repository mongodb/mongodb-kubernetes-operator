## MongoDB Agent Docker image

This folder contains the `Dockerfile` used to build [https://quay.io/repository/mongodb/mongodb-agent](https://quay.io/repository/mongodb/mongodb-agent). To build and push a new image to quay.io, use the following commands:

```
docker build . --build-arg agent_version=${agent_version} --build-arg tools_version=${tools_version} -t "quay.io/mongodb/mongodb-agent-ubi:${agent_version}"
docker push "quay.io/mongodb/mongodb-agent-ubi:${agent_version}"
```