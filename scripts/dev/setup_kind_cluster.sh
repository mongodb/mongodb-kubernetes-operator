#!/usr/bin/env bash
set -Eeou pipefail

# create the kind network early unless it already exists.
# it would normally be created automatically by kind but we
# need it earlier to get the IP address of our registry.
docker network create kind || true

# adapted from https://kind.sigs.k8s.io/docs/user/local-registry/
# create registry container unless it already exists
reg_name='kind-registry'
reg_port='5000'
running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
  docker run \
    -d --restart=always -p "${reg_port}:${reg_port}" --name "${reg_name}" --network kind \
    registry:2
fi

# find registry IP inside the kind network
ip="$(docker inspect kind-registry -f '{{.NetworkSettings.Networks.kind.IPAddress}}')"

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --kubeconfig ~/.kube/kind --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${ip}:${reg_port}"]
EOF

# Document the local registry (from  https://kind.sigs.k8s.io/docs/user/local-registry/)
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
