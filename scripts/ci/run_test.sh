#!/usr/bin/env bash


pip3 install -r ./requirements.txt
python3 ./scripts/dev/e2e.py --test ${test:?} --tag ${version_id:?} --config_file ./scripts/ci/config.json 
