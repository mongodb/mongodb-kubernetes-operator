#!/usr/bin/env bash

# shellcheck disable=SC1091
. venv/bin/activate
if [ -z "${clusterwide:-}" ]; then
    python3 ./scripts/dev/e2e.py --test "${test:?}" --tag "${version_id:?}" --config_file ./scripts/ci/config.json --distro "${distro:?}"
else
    python3 ./scripts/dev/e2e.py --test "${test:?}" --tag "${version_id:?}" --config_file ./scripts/ci/config.json --cluster-wide --distro "${distro:?}"
fi
