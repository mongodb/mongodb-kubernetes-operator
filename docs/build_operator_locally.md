# Quick start for building and deploy the operator locally

This document contains a quickstart guide to build and deploy the operator locally.


## Prerequisites
This guide assumes that you have already installed the following tools:

* [Kind](https://kind.sigs.k8s.io/)
* [Docker](https://www.docker.com/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)


## Steps

* Create a local kubernetes cluster and start a local registry by running

```sh
./scripts/dev/setup_kind_cluster.sh
```

* Build and deploy the operator:

```sh
python ./scripts/dev/build_and_deploy_operator
```

Note: this will build and push the operator at `repo_url/mongodb-kubernetes-operator`, where `repo_url` is extracted from the [dev config file](./contributing.md#developing-locally)

* Change the [operator yaml file](../deploy/operator.yaml) `image` field to have the image you just built

* You can now deploy your resources following the [docs](../docs)
