# Quick start for building and deploy the operator locally

This document contains a quickstart guide to build and deploy the operator locally.


## Prerequisites
This guide assumes that you have already prepared [Python virtual env](contributing.md#python-environment) and installed the following tools:

* [Kind](https://kind.sigs.k8s.io/)
* [Docker](https://www.docker.com/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)


## Steps

1. Create a local kubernetes cluster and start a local registry by running

```sh
./scripts/dev/setup_kind_cluster.sh -n test-cluster
```

2. Alternatively create a local cluster and set current kubectl context to it.
```sh
./scripts/dev/setup_kind_cluster.sh -en test-cluster
```

3. Run the following to get kind credentials and switch current context to the newly created cluster:

```sh
kind export kubeconfig --name test-cluster
# should return kind-test-cluster
kubectl config current-context
# should have test-cluster-control-plane node listed
kubectl get nodes
```

4. If you didn't clone the repository with `--recurse-submodules` flag you will need to download the helm-chart submodule locally by running the following command:
```sh
git submodule update --init
```


5. Build and deploy the operator. Also add `IMG_BUILD_ARGS=--insecure` as described [here](contributing.md#deploying-the-operator) if necessary:

```sh
# builds all required images and then deploys the operator
make all-images deploy
```

Note: this will build and push the operator at `repo_url/mongodb-kubernetes-operator`, where `repo_url` is extracted from the [dev config file](./contributing.md#developer-configuration)

6. Change the `image` field in the [manager.yaml](../config/manager/manager.yaml) file to have the image you just built

7. You can now deploy your resources following the [docs](../docs/README.md)
