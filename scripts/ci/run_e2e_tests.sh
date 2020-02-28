#!/usr/bin/env bash

#set -o nounset
#set -xeo pipefail

operator-sdk test local ./test/e2e --kubeconfig "${KUBECONFIG}" --namespace default --up-local