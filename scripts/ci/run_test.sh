#!/usr/bin/env bash

SKIP_CLEANUP="1" python ./scripts/dev/e2e.py --test ${test:?} --tag ${version_id:?} --config_file ./scripts/ci/config.json 

