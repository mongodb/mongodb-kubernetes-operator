#!/usr/bin/env bash

namespace=$1
replicaset_name=$2
secret_name=${replicaset_name}-config

if [[ "${namespace}" == "" || "${replicaset_name}" == "" ]]; then
  echo "Edit automation config secret for given replicaset."
  echo "It looks for the secret named '<replicaset_name>-secret' in the given namespace."
  echo "Requires jq to be installed and uses current kubectl context."
  echo
  echo "Usage:"
  printf "\t%s <namespace> <replicaset_name>\n" "$(basename "$0")"
  printf "\tEDITOR=<custom editor> %s <namespace> <replicaset_name> to edit cluster config with a different editor.\n" "$(basename "$0")"
  exit 1
fi

cluster_config_file=$(mktemp ./edit_cluster_config.sh.cluster_config.XXXXX)
# rename to have .json extension for syntax highlighting in the editor
mv "${cluster_config_file}" "${cluster_config_file}.json"
cluster_config_file="${cluster_config_file}.json"
cluster_config_file_base64="${cluster_config_file}.base64"

function cleanup() {
  rm -f "${cluster_config_file}" "${cluster_config_file_base64}"
}
trap cleanup EXIT

function get_secret() {
  local namespace=$1
  local secret_name=$2
  kubectl get secret "${secret_name}" -n "${namespace}" -o json | jq -r '.data."cluster-config.json"' | base64 -D
}

echo "Saving config to a temporary file: ${cluster_config_file}"
get_secret "${namespace}" "${secret_name}" | jq . -r >"${cluster_config_file}"
error_code=$?

if [[ ${error_code} != 0 ]]; then
  echo "Cluster config is invalid, edit without parsing with jq:"
  get_secret "${namespace}" "${secret_name}" >"${cluster_config_file}"
fi

if [[ "${EDITOR}" == "" ]]; then
  EDITOR=vim
fi

old_config=$(cat "${cluster_config_file}")
while true; do
  ${EDITOR} "${cluster_config_file}"
  new_config=$(jq . < "${cluster_config_file}")
  error_code=$?
  if [[ ${error_code} != 0 ]]; then
    read -n 1 -rsp $"Press any key to continue editing or ^C to abort..."
    echo
    continue
  fi
  break
done

if diff -q <(echo -n "${old_config}") <(echo -n "${new_config}"); then
  echo "No changes made to cluster config."
  exit 0
else
  echo "Cluster config was changed with following diff:"
  diff --normal <(echo -n "${old_config}") <(echo -n "${new_config}")
fi

base64 < "${cluster_config_file}" > "${cluster_config_file_base64}"

# shellcheck disable=SC2086
patch=$(cat <<EOF | jq -rc
[
 { "op"   : "replace",
   "path" : "/data/cluster-config.json",
   "value" : "$(cat ${cluster_config_file_base64})"
 }
]
EOF
)

kubectl patch secret -n "${namespace}" "${secret_name}" --type='json' -p="${patch}"