#!/bin/sh

BINDIR="${workdir}/bin"
mkdir -p "${BINDIR}" || true

# Store the lowercase name of Operating System
os=$(uname | tr '[:upper:]' '[:lower:]')
# This should be changed when needed
latest_version="v0.7.0"

mkdir -p "${workdir}/bin/"
echo "Saving kind to ${workdir}/bin"
curl --retry 3 --silent -L "https://github.com/kubernetes-sigs/kind/releases/download/${latest_version}/kind-${os}-amd64" -o kind

chmod +x kind
mv kind "${BINDIR}"
echo "Installed kind in ${BINDIR}"
