#!/bin/sh

docker --version
python scripts/dev/dockerfile_generator.py "python_formatting" > Dockerfile_python_formatting
DOCKER_BUILDKIT=1 docker build . -f Dockerfile_python_formatting -t "python_formatting:${version_id:?}"
docker run "python_formatting:${version_id:?}"
