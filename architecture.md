# MongoDB Community Operator Architecture

--------------------

## Table of Contents

- [MongoDB Docker Images](#mongodb-docker-images)
- [Example: MongoDB Version Upgrade](#example:-mongodb-version-upgrade)

--------------------

You create and update MongoDB resources by defining a MongoDB resource definition. When you apply the MongoDB resource definition to your Kubernetes environment, the Operator:

1. Creates a [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) that contains one [pod](https://kubernetes.io/docs/concepts/workloads/pods/pod-overview/) for each [replica set](https://docs.mongodb.com/manual/replication/) member. 
2. Creates two [containers](https://kubernetes.io/docs/concepts/containers/overview/) in each pod: 
   - A container for the [`mongod`](https://docs.mongodb.com/manual/reference/program/mongod/index.html) process binary.
   - A container for the Automation Agent. 
3. Writes the Automation configuration and initiates the Automation Agent, which in turn launches the `mongod` process according to your MongoDB resource definition.

While the Kubernetes Operator handles startup of the StatefulSet, the MongoDB Automation Agent handles configuring, stopping, and restarting the `mongod` process. The Automation Agent periodically polls the `mongod` to determine status and can deploy changes as needed. 

This architecure maximizes use of the Automation Agent while integrating naturally with Kubernetes to produce a number of benefits:

- Pods are defined as StatefulSets so they benefit from stable identities.
- Because the design is based on containers and not binaries, you can run any MongoDB container. This allows you to:
  - Use your preferred Linux distrobution inside the container.
  - Update operating system packages on your own schedule.
- Containers are immutable and have a single responsibility or process, which allows you to:
  - Describe and understand each container.
  - Configure resources independently for easier debugging and triage.
  - Inspect resources independently, including tailing the logs.
  - Expose the state of each container.
- You can upgrade the Operator without restarting either the database or the Automation Agent containers.
- You can set up a MongoDB Kubernetes cluster offline once you download the Docker containers for the database and Automation Agent.

## MongoDB Docker Images

MongoDB images are available on [Docker Hub](https://hub.docker.com/_/mongo?tab=tags&page=1&ordering=last_updated).

## Example: MongoDB Version Upgrade

The MongoDB Community Kubernetes Operator uses the Automation Agent to efficiently handle rolling upgrades. The StatefulSet is configured to block Kubernetes from performing native rolling upgrades because the native process can trigger multiple  re-elections in your MongoDB cluster. 

When you update the MongoDB version in your resource definition and reapply it to your Kubernetes environment, the Operator initiates a rolling upgrade:

1. The Operator updates the [image](https://kubernetes.io/docs/concepts/containers/images/) specification to the new version of MongoDB and writes a new Automation Agent configuration to each pod.
2. The Automation Agent chooses the first pod to upgrade and stops the `mongod` process using a local connection and [`db.shutdownServer`](https://docs.mongodb.com/manual/reference/method/db.shutdownServer/#db.shutdownServer).
3. A pre-stop hook on the database container checks the state of the Automation Agent. If the Automation Agent expects the `mongod` process to start with a new version, the hook uses a Kubernetes API call to delete the pod.
4. The Kubernetes Controller downloads the target version of MongoDB from its default docker registry and restarts the pod with the target version of `mongod` in the database container.
5. The Automation Agent starts. It finds and checks the target version of the new `mongod`, then generates the configuration file for the `mongod` process.
6. The `mongod` process finds the configuration file from the Automation Agent and starts.
7. The Automation Agent reaches goal state.
8. The Automation Agent chooses the next pod to upgrade and repeats the process until all pods are upgraded.

This upgrade process allows the Automation Agent to:

- Perform pre-conditions
- Upgrade the secondaries first
- Wait for the secondaries' oplogs to catch up before triggering an election
- Upgrade quickly for large replica sets
- Consider voting nodes
- Ensure a replica set is always avialable throughout the entire upgrade process
