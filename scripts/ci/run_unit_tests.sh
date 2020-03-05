#!/bin/sh

docker build . -f docker/Dockerfile.unittest -t unit-tests:${version_id}
docker run unit-tests:${version_id}
