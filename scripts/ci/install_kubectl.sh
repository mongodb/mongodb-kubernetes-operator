#!/usr/bin/env bash

BINDIR="${workdir}/bin"
mkdir -p "${BINDIR}" || true

echo "Downloading kubectl v1.15.4"
curl -s -LO https://storage.googleapis.com/kubernetes-release/release/v1.15.4/bin/linux/amd64/kubectl
chmod +x kubectl
mv kubectl $BINDIR