#!/usr/bin/env bash

. venv/bin/activate
pip3 install black
black --check `find . -type f -not -path '*venv*' -name '*.py'`
