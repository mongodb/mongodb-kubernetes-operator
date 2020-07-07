#!/usr/bin/env bash

. venv/bin/activate
python3 ./scripts/dev/e2e.py --test "${test:?}" --tag "${version_id:?}" --config_file ./scripts/ci/config.json 

