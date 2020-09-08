#!/usr/bin/env bash

# shellcheck disable=SC1091
. venv/bin/activate
python3 scripts/dev/release_image.py --old_repo_url "${old_image:-}" --new_repo_url "${new_image:-}" --old_tag "${version_id:-}" --labels "{\"quay.expires-after\":\"never\"}" --username "${quay_user_name:-}" --password "${quay_password:-}" --registry https://quay.io/organization/mongodb --release_file "./release.json" --image_type "${image_type:-}"
