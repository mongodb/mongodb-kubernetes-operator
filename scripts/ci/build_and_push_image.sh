#!/bin/sh

echo ${quay_password} | docker login -u=${quay_user_name} quay.io --password-stdin

python scripts/dev/dockerfile_generator.py ${image_type} > Dockerfile
docker build . -f Dockerfile -t ${image}
docker push ${image}
