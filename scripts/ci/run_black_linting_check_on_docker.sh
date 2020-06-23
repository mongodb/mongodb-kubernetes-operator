#!/bin/sh

python scripts/dev/dockerfile_generator.py "python_formatting" > Dockerfile
docker build . -f Dockerfile -t "python_formatting:${version_id:?}"
docker run "python_formatting:${version_id:?}"
