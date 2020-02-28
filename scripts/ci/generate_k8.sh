#!/usr/bin/env bash

set -o nounset
set -xeo pipefail

# GOROOT needs to be explicitly set otherwise we get the error
# time="2020-02-24T17:52:08Z" level=info msg="Running deepcopy code-generation for Custom Resource group versions: [mongodb:[v1], ]\n"
# deepcopy.go:885] Hit an unsupported type invalid type for invalid type, from ./pkg/apis/mongodb/v1.MongoDB
#GOROOT="/opt/golang/go1.13" operator-sdk generate k8s
operator-sdk generate k8s
#echo "using GOROOT=${GOROOT}"
#go version
#go env
#
#unset GOPATH
#operator-sdk generate k8s
