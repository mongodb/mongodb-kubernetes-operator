#!/bin/sh

files=`find . -type f -name '*.py'`
script="./scripts/ci/run_black_formatting_check.sh"
python scripts/dev/dockerfile_generator.py "python_formatting" --adds "$files" --script_location $script >  Dockerfile_python_formatting
DOCKER_BUILDKIT=1 docker build . -f Dockerfile_python_formatting -t "python_formatting:${version_id:?}"
docker run "python_formatting:${version_id:?}"
