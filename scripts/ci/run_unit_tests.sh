#!/bin/sh

python docker/dockerfile_generator.py "unittest" > Dockerfile
docker build . -f Dockerfile -t unit-tests:${version_id}
docker run unit-tests:${version_id}
