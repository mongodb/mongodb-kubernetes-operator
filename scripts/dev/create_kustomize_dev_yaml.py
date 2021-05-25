#!/usr/bin/env python

import sys
import yaml

from dev_config import load_config


def main() -> int:

    # TODO: CLOUDP-86212 this script should be removed when Helm replaces Kustomize is introduced.
    dev_config = load_config()
    dev_yaml = {
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": {
            "name": "mongodb-kubernetes-operator",
            "namespace": dev_config.namespace,
        },
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "name": "mongodb-kubernetes-operator",
                            "env": [
                                {
                                    "name": "AGENT_IMAGE",
                                    "value": f"{dev_config.repo_url}/{dev_config.agent_image}:latest",
                                },
                                {
                                    "name": "VERSION_UPGRADE_HOOK_IMAGE",
                                    "value": f"{dev_config.repo_url}/{dev_config.version_upgrade_hook_image}:latest",
                                },
                                {
                                    "name": "READINESS_PROBE_IMAGE",
                                    "value": f"{dev_config.repo_url}/{dev_config.readiness_probe_image}:latest",
                                },
                                {
                                    "name": "WATCH_NAMESPACE",
                                    "value": dev_config.namespace,
                                    "valueFrom": None,
                                },
                            ],
                        }
                    ]
                }
            }
        },
    }

    # create the temporary patch that should be used to deploy dev images.
    with open("config/dev/custom-env.yaml", "w+") as f:
        f.write(yaml.dump(dev_yaml))

    return 0


if __name__ == "__main__":
    sys.exit(main())
