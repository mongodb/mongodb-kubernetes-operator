#!/usr/bin/env bash

if ! command -v telepresence &> /dev/null; then \
  echo "Telepresence not found, installing now"
  sudo curl -fL https://app.getambassador.io/download/tel2/"$(go env GOHOSTOS)"/"$(go env GOHOSTARCH)"/latest/telepresence -o /tmp/telepresence
  sudo mv /tmp/telepresence /usr/local/bin/telepresence
  sudo chmod a+x /usr/local/bin/telepresence
else
	echo "Telepresence already installed."
fi
