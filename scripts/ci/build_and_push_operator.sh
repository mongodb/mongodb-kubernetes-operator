#!/bin/sh

echo ${quay_password} | docker login -u=${quay_user_name} quay.io --password-stdin

docker build . -f Dockerfile-operator quay.io/mongodb/community-operator-dev:${revision}

docker push quay.io/mongodb/community-operator-dev:${revision}
