#!/usr/bin/env bash
set -Eeou pipefail

####
# This file is copy-pasted from https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/scripts/dev/setup_kind_cluster.sh
# Do not edit !!!
####

function usage() {
  echo "Deploy local registry and create kind cluster configured to use this registry. Local Docker registry is deployed at localhost:5000.

Usage:
  setup_kind_cluster.sh [-n <cluster_name>] [-r]
  setup_kind_cluster.sh [-h]
  setup_kind_cluster.sh [-n <cluster_name>] [-e] [-r]

Options:
  -n <cluster_name>    (optional) Set kind cluster name to <cluster_name>. Creates kubeconfig in ~/.kube/<cluster_name>. The default name is 'kind' if not set.
  -e                   (optional) Export newly created kind cluster's credentials to ~/.kube/<cluster_name> and set current kubectl context.
  -h                   (optional) Shows this screen.
  -r                   (optional) Recreate cluster if needed
  -p <pod network>     (optional) Network reserved for Pods, e.g. 10.244.0.0/16
  -s <service network> (optional) Network reserved for Services, e.g. 10.96.0.0/16
"
  exit 0
}

cluster_name=${CLUSTER_NAME:-"kind"}
export_kubeconfig=0
recreate=0
pod_network="10.244.0.0/16"
service_network="10.96.0.0/16"
while getopts ':p:s:n:her' opt; do
    case $opt in
      (n)   cluster_name=$OPTARG;;
      (e)   export_kubeconfig=1;;
      (r)   recreate=1;;
      (p)   pod_network=$OPTARG;;
      (s)   service_network=$OPTARG;;
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
  docker run -d --restart=always -p "127.0.0.1:${reg_port}:5000" --network kind --name "${reg_name}" registry:2
fi

if [ "${recreate}" != 0 ]; then
  kind delete cluster --name "${cluster_name}" || true
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --name "${cluster_name}" --kubeconfig "${kubeconfig_path}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  podSubnet: "${pod_network}"
  serviceSubnet: "${service_network}"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
EOF

# Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${reg_port} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${reg_port}"
for node in $(kind get nodes --name "${cluster_name}"); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${reg_name}:5000"]
EOF
done

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
