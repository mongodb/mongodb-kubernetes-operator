#!/usr/bin/env bash

set +x

# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin

python3 scripts/dev/dockerfile_generator.py "${image_type:?}" > Dockerfile

# Providing the quay.expires-after configures quay to delete this image after the provided amount of time
docker build . --label "quay.expires-after=${expire_after:-never}" -f Dockerfile -t "${image:?}"
docker push "${image:?}"
