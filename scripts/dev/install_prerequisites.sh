#!/usr/bin/env bash

if ! command -v telepresence &> /dev/null; then \
  echo "Telepresence not found, installing now"
  if [[ $(uname -m) == 'arm64' ]]; then
      echo "arm64"
      sudo curl -fL https://app.getambassador.io/download/tel2/darwin/arm64/latest/telepresence -o /usr/local/bin/telepresence
  else
      echo "amd64"
      sudo curl -fL https://app.getambassador.io/download/tel2/darwin/amd64/latest/telepresence -o /usr/local/bin/telepresence
  fi
  sudo chmod a+x /usr/local/bin/telepresence
else
	echo "Telepresence already installed."
fi
