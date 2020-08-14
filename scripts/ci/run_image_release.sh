#!/usr/bin/env bash

# shellcheck disable=SC1091
. venv/bin/activate
python3 scripts/ci/release_image.py --old_repo_url quay.io/mongodb/community-operator-dev --new_repo_url=quay.io/repository/mongodb/mongodb-kubernetes-operator --old_tag ${version_id} --labels "{\"quay.expires.after-after\":\"never\"}" --username ${quay_user_name} --password ${quay_password} --registry https://quay.io/organization/mongodb --release_file "./release.json"
