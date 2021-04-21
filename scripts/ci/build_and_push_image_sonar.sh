#!/usr/bin/env bash

set +x

# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin

# shellcheck disable=SC2154
if [ -n "${release}" ]; then
  # build and push image and also retag and release
  python3 pipeline.py --image-name "${image_name}" --release
else
  # just build and push image
  python3 pipeline.py --image-name "${image_name}"
fi
