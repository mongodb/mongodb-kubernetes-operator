#!/usr/bin/env python3
import sys
from typing import Dict
import os.path


from dev_config import load_config, DevConfig, Distro


def _get_e2e_test_envs(dev_config: DevConfig) -> Dict[str, str]:
    """
    _get_e2e_test_envs returns a dictionary of all the required environment variables
    that need to be set in order to run a local e2e test.

    :param dev_config: The local dev config
    :return: A diction of env vars to be set
    """
    cleanup = False
    if len(sys.argv) > 1:
        cleanup = sys.argv[1] == "true"
    return {
        "ROLE_DIR": dev_config.role_dir,
        "DEPLOY_DIR": dev_config.deploy_dir,
        "OPERATOR_IMAGE": f"{dev_config.repo_url}/{dev_config.operator_image}",
        "VERSION_UPGRADE_HOOK_IMAGE": f"{dev_config.repo_url}/{dev_config.version_upgrade_hook_image}",
        "AGENT_IMAGE": f"{dev_config.repo_url}/{dev_config.agent_image}",
        "TEST_DATA_DIR": dev_config.test_data_dir,
        "TEST_NAMESPACE": dev_config.namespace,
        "READINESS_PROBE_IMAGE": f"{dev_config.repo_url}/{dev_config.readiness_probe_image}",
        "PERFORM_CLEANUP": "true" if cleanup else "false",
        "WATCH_NAMESPACE": dev_config.namespace,
        "MONGODB_IMAGE": dev_config.mongodb_image_name,
        "MONGODB_REPO_URL": dev_config.mongodb_image_repo_url,
        "HELM_CHART_PATH": os.path.abspath("./helm-charts/charts/community-operator"),
        "MDB_IMAGE_TYPE": dev_config.image_type,
        "MDB_LOCAL_OPERATOR": dev_config.local_operator,
        "KUBECONFIG": dev_config.kube_config,
    }


# convert all values in config.json to env vars.
# this can be used to provide configuration for e2e tests.
def main() -> int:
    dev_config = load_config(distro=Distro.UBI)
    for k, v in _get_e2e_test_envs(dev_config).items():
        print(f"export {k.upper()}={v}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
