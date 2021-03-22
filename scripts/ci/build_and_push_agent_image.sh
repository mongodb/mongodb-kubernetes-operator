#!/usr/bin/env bash


# shellcheck disable=SC1091
. venv/bin/activate
echo "${quay_password:?}" | docker login "-u=${quay_user_name:?}" quay.io --password-stdin


# Providing the quay.expires-after configures quay to delete this image after the provided amount of time
<<<<<<< HEAD
docker build . --label "quay.expires-after=${expire_after:-never}" -f "${dockerfile_path:?}" -t "${image:?}" --build-arg tools_version="100.2.0" --build-arg agent_version="10.27.0.6772-1"
=======
docker build . --label "quay.expires-after=${expire_after:-never}" -f agent/Dockerfile -t "${image:?}" --build-arg tools_version="100.2.0" --build-arg agent_version="10.27.0.6772-1"
>>>>>>> master
docker push "${image:?}"
