#!/bin/sh

go run scripts/ci/e2e/e2e.go --kubeconfig ${KUBECONFIG} --operatorImage quay.io/mongodb/community-operator-dev:${version_id}
