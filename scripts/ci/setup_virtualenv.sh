#!/usr/bin/env bash

# shellcheck disable=SC1091
virtualenv --python /opt/python/3.7/bin/python3 ./venv
. venv/bin/activate

pip3 install -r ./requirements.txt
pip3 install git+https://${SONAR_GITHUB_TOKEN}@github.com/10gen/sonar.git@0.0.8
