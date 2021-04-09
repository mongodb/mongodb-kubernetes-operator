#!/usr/bin/env bash


# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin

python3 pipeline.py --image-name ${image_name} --labels "quay.expires-after=${expire_after:-never}"
