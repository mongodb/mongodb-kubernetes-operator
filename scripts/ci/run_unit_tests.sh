#!/bin/sh

# shellcheck disable=SC1091
. venv/bin/activate
python scripts/dev/dockerfile_generator.py "unittest" > Dockerfile
docker build . -f Dockerfile -t "unit-tests:${version_id:?}"
docker run "unit-tests:${version_id:?}"
