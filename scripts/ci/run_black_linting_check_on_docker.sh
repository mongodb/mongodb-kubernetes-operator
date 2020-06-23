#!/bin/sh

python scripts/dev/dockerfile_generator.py "linting" > Dockerfile
docker build . -f Dockerfile -t "linting:${version_id:?}"
docker run "linting:${version_id:?}"
