# MongoDB Community Kubernetes Operator Architecture

The MongoDB Community Kubernetes Operator is a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and a [Controller](https://kubernetes.io/docs/concepts/architecture/controller/).

## Table of Contents

- [Cluster Configuration](#cluster-configuration)
- [Example: MongoDB Version Upgrade](#example-mongodb-version-upgrade)
- [MongoDB Docker Images](#mongodb-docker-images)

## Cluster Configuration

You create and update MongoDB resources by defining a MongoDB resource definition. When you apply the MongoDB resource definition to your Kubernetes environment, the Operator:

1. Creates a [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) that contains one [pod](https://kubernetes.io/docs/concepts/workloads/pods/pod-overview/) for each [replica set](https://docs.mongodb.com/manual/replication/) member. 
1. Creates two [containers](https://kubernetes.io/docs/concepts/containers/overview/) in each pod:

   - A container for the [`mongod`](https://docs.mongodb.com/manual/reference/program/mongod/index.html) process binary. </br>
     `mongod` is the primary daemon process for the MongoDB system. It handles data requests, manages data access, and performs background management operations.

   - A container for the Automation Agent. </br>
     The Automation Agent handles configuring, stopping, and restarting the `mongod` process. The Automation Agent periodically polls the `mongod` to determine status and can deploy changes as needed. 
1. Writes the Automation configuration as a [ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/) and mounts it to each pod. 
1. Initiates the Automation Agent, which in turn creates the database configuration and launches the `mongod` process according to your MongoDB resource definition.

<!--
<img src="" alt="Architecure diagram of the MongoDB Community Kubernetes Operator">
-->

This architecture maximizes use of the Automation Agent while integrating naturally with Kubernetes to produce a number of benefits.

- MongoDB containers are not tied to the Operator or the Agent, so you can:
  - Run any MongoDB container.
  - Use your preferred Linux distribution inside the container.
  - Update operating system packages on your own schedule.
- Containers are immutable and have a single responsibility or process, so you can:
  - Describe and understand each container.
  - Configure resources independently for easier debugging and triage.
  - Inspect resources independently, including tailing the logs.
  - Expose the state of each container.
- Pods are defined as StatefulSets so they benefit from stable identities.
- You can upgrade the Operator without restarting either the database or the Automation Agent containers.
- You can set up a MongoDB Kubernetes cluster offline once you download the Docker containers for the database and Automation Agent.

## Example: MongoDB Version Upgrade

The MongoDB Community Kubernetes Operator uses the Automation Agent to efficiently handle rolling upgrades. The Operator configures the StatefulSet to block Kubernetes from performing native rolling upgrades because the native process can trigger multiple  re-elections in your MongoDB cluster. 

When you update the MongoDB version in your resource definition and reapply it to your Kubernetes environment, the Operator initiates a rolling upgrade:

1. The Operator changes the StatefulSet [update strategy](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#update-strategies) from `RollingUpdate` to `OnDelete`.

1. The Operator updates the [image](https://kubernetes.io/docs/concepts/containers/images/) specification to the new version of MongoDB and writes a new Automation Agent configuration ConfigMap to each pod.

1. The Automation Agent chooses the first pod to upgrade and stops the `mongod` process using a local connection and [`db.shutdownServer`](https://docs.mongodb.com/manual/reference/method/db.shutdownServer/#db.shutdownServer).

1. A pre-stop hook on the database container checks the state of the Automation Agent. If the Automation Agent expects the `mongod` process to start with a new version, the hook uses a Kubernetes API call to delete the pod.

1. The Kubernetes Controller downloads the target version of MongoDB from its default docker registry and restarts the pod with the target version of `mongod` in the database container.

1. The Automation Agent starts. It checks the target version of the new `mongod`, then generates the configuration file for the `mongod` process.

1. The `mongod` process receives the configuration file from the Automation Agent and starts.

1. The Automation Agent reaches goal state.

1. The Automation Agent chooses the next pod to upgrade and repeats the process until all pods are upgraded.

1. The Operator changes the StatefulSet update strategy from `OnDelete` back to `RollingUpdate`.

<!--
<img src="" alt="Rolling upgrade flow diagram for the MongoDB Community Kubernetes Operator">
-->

This upgrade process allows the Automation Agent to:

- Perform pre-conditions
- Upgrade the secondaries first
- Wait for the secondaries' oplogs to catch up before triggering an election
- Upgrade quickly for large replica sets
- Consider voting nodes
- Ensure a replica set is always available throughout the entire upgrade process

## MongoDB Docker Images

MongoDB images are available on [Docker Hub](https://hub.docker.com/_/mongo?tab=tags&page=1&ordering=last_updated).
