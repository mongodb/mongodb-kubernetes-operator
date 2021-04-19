#!/usr/bin/env bash


# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin

# shellcheck disable=SC2154
python3 pipeline.py --image-name "${image_name}"
