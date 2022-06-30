#!/usr/bin/env bash
set -Eeou pipefail

function usage() {
  echo "Deploy local registry and create kind cluster configured to use this registry. Local Docker registry is deployed at localhost:5000.

Usage:
  setup_kind_cluster.sh [-n <cluster_name>] [-r]
  setup_kind_cluster.sh [-h]
  setup_kind_cluster.sh [-n <cluster_name>] [-e] [-r]

Options:
  -n <cluster_name>   (optional) Set kind cluster name to <cluster_name>. Creates kubeconfig in ~/.kube/<cluster_name>. The default name is 'kind' if not set.
  -e                  (optional) Export newly created kind cluster's credentials to ~/.kube/<cluster_name> and set current kubectl context.
  -h                  (optional) Shows this screen.
  -r                  (optional) Recreate cluster if needed
"
  exit 0
}

cluster_name=${CLUSTER_NAME:-"kind"}
export_kubeconfig=0
recreate=0
while getopts ':n:her' opt; do
    case $opt in
      (n)   cluster_name=$OPTARG;;
      (e)   export_kubeconfig=1;;
      (r)   recreate=1;;
      (h)   usage;;
      (*)   usage;;
    esac
done
shift "$((OPTIND-1))"

kubeconfig_path="$HOME/.kube/${cluster_name}"

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
  docker run -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" registry:2
fi

if [ "${recreate}" != 0 ]; then
  kind delete cluster --name "${cluster_name}" || true
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --name "${cluster_name}" --kubeconfig "${kubeconfig_path}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:${reg_port}"]
EOF

# connect the registry to the cluster network if not already connected
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  docker network connect "kind" "${reg_name}"
fi

# Document the local registry (from  https://kind.sigs.k8s.io/docs/user/local-registry/)
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply --kubeconfig "${kubeconfig_path}" -f -
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

if [[ "${export_kubeconfig}" == "1" ]]; then
  kind export kubeconfig --name "${cluster_name}"
fi