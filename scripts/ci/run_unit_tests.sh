#!/bin/sh

docker build . -f docker/Dockerfile.unittest -t unit-tests:${revision}
docker run unit-tests:${revision}
