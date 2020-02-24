#!/bin/sh

set -o nounset
set -xeo pipefail

BINDIR="${workdir}/bin"
RELEASE_VERSION=v0.15.1

echo "Downloading Operator SDK version: ${RELEASE_VERSION}"
curl --retry 3 --silent -L https://github.com/operator-framework/operator-sdk/releases/download/${RELEASE_VERSION}/operator-sdk-${RELEASE_VERSION}-x86_64-linux-gnu -o operator-sdk
chmod +x operator-sdk
mkdir -p "${BINDIR}"
mv operator-sdk "${workdir}/bin"
echo "Installed Operator SDK to ${BINDIR}"