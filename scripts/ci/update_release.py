#!/usr/bin/env python3

import json
import sys
import yaml
from typing import Dict

RELATIVE_PATH_TO_MANAGER_YAML = "config/manager/manager.yaml"


def _load_release() -> Dict:
    with open("release.json", "r") as f:
        return json.loads(f.read())


def _load_manager_yaml() -> Dict:
    with open(RELATIVE_PATH_TO_MANAGER_YAML, "r") as f:
        return yaml.safe_load(f.read())


def _replace_tag(image: str, new_tag: str) -> str:
    split_image = image.split(":")
    return f"{split_image[0]}:{new_tag}"


def _update_operator_deployment(operator_deployment: Dict, release: Dict) -> None:
    operator_container = operator_deployment["spec"]["template"]["spec"]["containers"][0]
    operator_container["image"] = _replace_tag(operator_container["image"], release["mongodb-kubernetes-operator"])
    operator_envs = operator_container["env"]
    for env in operator_envs:
        if env["name"] == "VERSION_UPGRADE_HOOK_IMAGE":
            env["value"] = _replace_tag(env["value"], release["version-upgrade-hook"])
        if env["name"] == "READINESS_PROBE_IMAGE":
            env["value"] = _replace_tag(env["value"], release["readiness-probe"])
        if env["name"] == "AGENT_IMAGE":
            env["value"] = _replace_tag(env["value"], release["mongodb-agent"]["version"])


def _update_manager_yaml(operator_deployment: Dict):
    with open(RELATIVE_PATH_TO_MANAGER_YAML, "w+") as f:
        return yaml.dump(operator_deployment, f)


def main() -> int:
    release = _load_release()
    manager = _load_manager_yaml()
    _update_operator_deployment(manager, release)
    _update_manager_yaml(manager)
    return 0


if __name__ == "__main__":
    sys.exit(main())
