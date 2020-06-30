#!/usr/bin/env bash

. venv/bin/activate
black --check `find . -type f -not -path '*venv*' -name '*.py'`
