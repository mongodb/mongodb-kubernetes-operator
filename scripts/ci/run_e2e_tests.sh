#!/usr/bin/env bash

set -o nounset
set -xeo pipefail

go mod vendor
operator-sdk test local ./test/e2e --kubeconfig "${KUBECONFIG}" --namespace default