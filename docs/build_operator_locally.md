# Quick start for building and deploy the operator locally

This document contains a quickstart guide to build and deploy the operator locally.


## Prerequisites
This guide assumes that you have already installed the following tools:

* [Kind](https://kind.sigs.k8s.io/)
* [Docker](https://www.docker.com/)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)


## Steps

1. Create a local kubernetes cluster and start a local registry by running

```sh
./scripts/dev/setup_kind_cluster.sh
```

2. Set the kind kubernetes context if you have not already done so:
```bash
export KUBECONFIG=~/.kube/kind
```

3. Run the following to get kind credentials:

```sh
kind export kubeconfig
# check it worked by running:
kubectl cluster-info --context kind-kind --kubeconfig $KUBECONFIG
```

4. Build and deploy the operator:

```sh
python ./scripts/dev/build_and_deploy_operator
```

Note: this will build and push the operator at `repo_url/mongodb-kubernetes-operator`, where `repo_url` is extracted from the [dev config file](./contributing.md#developing-locally)

5. Change the [operator yaml file](../deploy/operator.yaml) `image` field to have the image you just built

6. You can now deploy your resources following the [docs](../docs/README.md)


## Troubleshooting
If you run into an issue in step 1, you can try the following steps as workaround:
1. Manually build the operator Dockerfile
```sh
python ./scripts/dev/dockerfile_generator.py > Dockerfile
```

2. Build the image
```sh
docker build . -t localhost:5000/mongodb-kubernetes-operator
```

3. Push the image
```sh
docker push localhost:5000/mongodb-kubernetes-operator
```
