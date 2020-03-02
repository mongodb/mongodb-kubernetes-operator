#!/usr/bin/env bash

set -o nounset
set -xeo pipefail

go mod vendor
# we run the tests in verbose mode to ensure we get all test logs displayed
operator-sdk test local ./test/e2e --kubeconfig "${KUBECONFIG}" --namespace default --verbose
