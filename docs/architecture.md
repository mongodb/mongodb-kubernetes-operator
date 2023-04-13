# MongoDB Community Kubernetes Operator Architecture

The MongoDB Community Kubernetes Operator is a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and a [Controller](https://kubernetes.io/docs/concepts/architecture/controller/).

## Table of Contents

- [Cluster Configuration](#cluster-configuration)
- [Example: MongoDB Version Upgrade](#example-mongodb-version-upgrade)
- [MongoDB Docker Images](#mongodb-docker-images)

## Cluster Configuration

You create and update MongoDBCommunity resources by defining a MongoDBCommunity resource definition. When you apply the MongoDBCommunity resource definition to your Kubernetes environment, the Operator:

1. Creates a [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) that contains one [pod](https://kubernetes.io/docs/concepts/workloads/pods/pod-overview/) for each [replica set](https://www.mongodb.com/docs/manual/replication/) member.
1. Writes the Automation configuration as a [Secret](https://kubernetes.io/docs/concepts/configuration/secret/) and mounts it to each pod.
1. Creates one [init container](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) and two [containers](https://kubernetes.io/docs/concepts/containers/overview/) in each pod:

   - An init container which copies the `cmd/versionhook` binary to the main `mongod` container. This is run before `mongod` starts to handle [version upgrades](#example-mongodb-version-upgrade).

   - A container for the [`mongod`](https://www.mongodb.com/docs/manual/reference/program/mongod/index.html) process binary. `mongod` is the primary daemon process for the MongoDB system. It handles data requests, manages data access, and performs background management operations.

   - A container for the MongoDB Agent. The Automation function of the MongoDB Agent handles configuring, stopping, and restarting the `mongod` process. The MongoDB Agent periodically polls the `mongod` to determine status and can deploy changes as needed.

1. Creates several volumes:

   - `data-volume` which is [persistent](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) and mounts to `/data` on both the server and agent containers. Stores server data as well as `automation-mongod.conf` written by the agent and some locks the agent needs.
   - `automation-config` which is mounted from the previously generated `Secret` to both the server and agent. Only lives as long as the pod.
   - `healthstatus` which contains the agent's current status. This is shared with the `mongod` container where it's used by the pre-stop hook. Only lives as long as the pod.

1. Initiates the MongoDB Agent, which in turn creates the database configuration and launches the `mongod` process according to your [MongoDBCommunity resource definition](../config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml).

<!--
<img src="" alt="Architecure diagram of the MongoDB Community Kubernetes Operator">
-->

This architecture maximizes use of the MongoDB Agent while integrating naturally with Kubernetes to produce a number of benefits.

- The database container is not tied to the lifecycle of the Agent container or to the Operator, so you can:
  - Use your preferred Linux distribution inside the container.
  - Update operating system packages on your own schedule.
  - Upgrade the Operator or Agent without affecting the database image or uptime of the MongoDB servers.
- Containers are immutable and have a single responsibility or process, so you can:
  - Describe and understand each container.
  - Configure resources independently for easier debugging and triage.
  - Inspect resources independently, including tailing the logs.
  - Expose the state of each container.
- Pods are defined as StatefulSets so they benefit from stable identities.
- You can upgrade the Operator without restarting either the database or the MongoDB Agent containers.
- You can set up a MongoDB Kubernetes cluster offline once you download the Docker containers for the database and MongoDB Agent.

## Example: MongoDB Version Upgrade

The MongoDB Community Kubernetes Operator uses the Automation function of the MongoDB Agent to efficiently handle rolling upgrades. The Operator configures the StatefulSet to block Kubernetes from performing native rolling upgrades because the native process can trigger multiple re-elections in your MongoDB cluster.

When you update the MongoDB version in your resource definition and reapply it to your Kubernetes environment, the Operator initiates a rolling upgrade:

1. The Operator changes the StatefulSet [update strategy](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#update-strategies) from `RollingUpdate` to `OnDelete`.

1. The Operator updates the [image](https://kubernetes.io/docs/concepts/containers/images/) specification to the new version of MongoDB and writes a new Automation configuration ConfigMap to each pod.

1. The MongoDB Agent chooses the first pod to upgrade and stops the `mongod` process using a local connection and [`db.shutdownServer`](https://www.mongodb.com/docs/manual/reference/method/db.shutdownServer/#db.shutdownServer).

1. Kubernetes will restart the `mongod` container causing the version change hook to run before the `mongod` process and check the state of the MongoDB Agent. If the MongoDB Agent expects the `mongod` process to start with a new version, the hook uses a Kubernetes API call to delete the pod.

1. The Kubernetes Controller downloads the target version of MongoDB from its default docker registry and restarts the pod with the target version of `mongod` in the database container.

1. The MongoDB Agent starts. It checks the target version of the new `mongod`, then generates the configuration file for the `mongod` process.

1. The `mongod` process receives the configuration file from the MongoDB Agent and starts.

1. The MongoDB Agent reaches goal state.

1. The MongoDB Agent chooses the next pod to upgrade and repeats the process until all pods are upgraded.

1. The Operator changes the StatefulSet update strategy from `OnDelete` back to `RollingUpdate`.

<!--
<img src="" alt="Rolling upgrade flow diagram for the MongoDB Community Kubernetes Operator">
-->

This upgrade process allows the MongoDB Agent to:

- Perform pre-conditions.
- Upgrade the secondaries first.
- Wait for the secondaries' oplogs to catch up before triggering an election.
- Upgrade quickly for large replica sets.
- Consider voting nodes.
- Ensure a replica set is always available throughout the entire upgrade process.

## MongoDB Docker Images

MongoDB images are available on [Docker Hub](https://hub.docker.com/_/mongo?tab=tags&page=1&ordering=last_updated).
