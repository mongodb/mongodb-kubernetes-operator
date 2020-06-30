#!/usr/bin/env bash

. venv/bin/activate
find . -type f -not -path '*venv*' -name '*.py' -exec python3 -m mypy --ignore-missing-imports {} +
