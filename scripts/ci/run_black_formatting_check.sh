#!/usr/bin/env bash

# shellcheck disable=SC1091
. venv/bin/activate
black --check "$(find . -type f -not -path '*venv*' -name '*.py')"
