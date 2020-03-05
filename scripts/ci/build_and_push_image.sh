#!/bin/sh

echo ${quay_password} | docker login -u=${quay_user_name} quay.io --password-stdin

docker build . -f ${dockerfile} -t ${image}
docker push ${image}
