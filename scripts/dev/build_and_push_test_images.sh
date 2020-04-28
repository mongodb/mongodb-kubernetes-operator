#!/usr/bin/env bash
set -Eeou pipefail

docker_file=$(mktemp)
python docker/dockerfile_generator.py e2e > "${docker_file}"
docker build . -t "${REPO_URL}/e2e" -f "${docker_file}" && docker push "${REPO_URL}/e2e"
rm "${docker_file}"

docker_file=$(mktemp)
python docker/dockerfile_generator.py testrunner > "${docker_file}"
docker build . -t "${REPO_URL}/test-runner" -f "${docker_file}" && docker push "${REPO_URL}/test-runner"
rm "${docker_file}"
