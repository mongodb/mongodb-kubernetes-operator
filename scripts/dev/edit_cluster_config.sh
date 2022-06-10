#!/usr/bin/env bash

namespace=$1
replicaset_name=$2

cluster_config_file=$(mktemp ./edit_cluster_config.sh.cluster_config.json.XXXXX)
cluster_config_file_base64="edit_cluster_config.sh.${cluster_config_file}.base64"

kubectl get secret "${replicaset_name}-config" -n "${namespace}" -o json | jq -r '.data."cluster-config.json"' | base64 -D | jq . -r >"${cluster_config_file}"

if [[ "${EDITOR}" == "" ]]; then
  EDITOR=vim
fi

${EDITOR} "${cluster_config_file}"
base64 < "${cluster_config_file}" > "${cluster_config_file_base64}"

patch=$(cat <<EOF | jq -rc
[
 { "op"   : "replace",
   "path" : "/data/cluster-config.json",
   "value" : "$(cat ${cluster_config_file_base64})"
 }
]
EOF
)

kubectl patch secret -n mongodb mdb2-config --type='json' -p="${patch}"

#rm "${secret_file}"
#rm "${secret_file_new}"