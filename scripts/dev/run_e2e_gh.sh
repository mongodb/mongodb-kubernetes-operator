#!/usr/bin/env bash
set -Eeou pipefail

test_name="${1}"
current_branch="$(git branch --show-current)"

gh workflow run e2e-dispatch.yml -f "test-name=${test_name}" --ref "${current_branch}"

run_id="$(gh run list --workflow=e2e-dispatch.yml | grep workflow_dispatch | grep -Eo "[0-9]+" | head -n 1)"

gh run view "${run_id}" --web
