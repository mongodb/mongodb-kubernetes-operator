#!/usr/bin/env bash

# shellcheck disable=SC1091
. venv/bin/activate
find . -type f -not -path '*venv*' -name '*.py' -exec python3 -m mypy --disallow-untyped-calls --disallow-untyped-defs --disallow-incomplete-defs --ignore-missing-imports {} +
