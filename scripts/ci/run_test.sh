#!/usr/bin/env bash


pip3 install -r ./requirements.txt
SKIP_CLEANUP="1" python3 ./scripts/dev/e2e.py --skip-operator-install 1 --skip-image-build 1 --test ${test:?} --tag ${version_id:?} --config_file ./scripts/ci/config.json
