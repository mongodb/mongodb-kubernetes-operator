#!/usr/bin/env bash

# shellcheck disable=SC1091
virtualenv --python /opt/python/3.7/bin/python3 ./venv
. venv/bin/activate
pip3 install -r ./requirements.txt
