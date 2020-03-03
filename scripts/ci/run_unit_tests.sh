#!/bin/sh

docker build . -f Dockerfile-unittest -t unit-tests:${revision}
docker run unit-tests:${revision}
