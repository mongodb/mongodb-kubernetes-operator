#!/usr/bin/env bash


# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin

python3 scripts/dev/dockerfile_generator.py "${image_type:?}" > Dockerfile
docker build . --build-arg quay_expiration="${expire_after:-never}" -f Dockerfile -t "${image:?}"
docker push "${image:?}"
