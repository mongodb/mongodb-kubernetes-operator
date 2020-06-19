#!/usr/bin/env bash

SKIP_CLEANUP="1" python3 ./scripts/dev/e2e.py --skip-image-build 1 --test ${test:?} --tag ${version_id:?} --config_file ./scripts/ci/config.json
