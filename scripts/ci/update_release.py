#!/usr/bin/env python3

import json
import sys
from typing import Dict, Callable

import ruamel.yaml

yaml = ruamel.yaml.YAML()


RELATIVE_PATH_TO_MANAGER_YAML = "config/manager/manager.yaml"
RELATIVE_PATH_TO_OPENSHIFT_MANAGER_YAML = "deploy/openshift/operator_openshift.yaml"

RELATIVE_PATH_TO_CHART_VALUES = "helm-charts/charts/community-operator/values.yaml"
RELATIVE_PATH_TO_CHART = "helm-charts/charts/community-operator/Chart.yaml"
RELATIVE_PATH_TO_CRD_CHART = "helm-charts/charts/community-operator-crds/Chart.yaml"


def _load_yaml_file(path: str) -> Dict:
    with open(path, "r") as f:
        return yaml.load(f.read())


def _dump_yaml(operator: Dict, path: str) -> None:
    with open(path, "w+") as f:
        yaml.dump(operator, f)


def update_and_write_file(path: str, update_function: Callable) -> None:
    release = _load_release()
    yaml_file = _load_yaml_file(path)
    update_function(yaml_file, release)
    _dump_yaml(yaml_file, path)


def _load_release() -> Dict:
    with open("release.json", "r") as f:
        return json.loads(f.read())


def _replace_tag(image: str, new_tag: str) -> str:
    split_image = image.split(":")
    return split_image[0] + ":" + new_tag


def update_operator_deployment(operator_deployment: Dict, release: Dict) -> None:
    operator_container = operator_deployment["spec"]["template"]["spec"]["containers"][
        0
    ]
    operator_container["image"] = _replace_tag(
        operator_container["image"], release["operator"]
    )
    operator_envs = operator_container["env"]
    for env in operator_envs:
        if env["name"] == "VERSION_UPGRADE_HOOK_IMAGE":
            env["value"] = _replace_tag(env["value"], release["version-upgrade-hook"])
        if env["name"] == "READINESS_PROBE_IMAGE":
            env["value"] = _replace_tag(env["value"], release["readiness-probe"])
        if env["name"] == "AGENT_IMAGE":
            env["value"] = _replace_tag(env["value"], release["agent"])


def update_chart_values(values: Dict, release: Dict) -> None:
    values["agent"]["version"] = release["agent"]
    values["versionUpgradeHook"]["version"] = release["version-upgrade-hook"]
    values["readinessProbe"]["version"] = release["readiness-probe"]
    values["operator"]["version"] = release["operator"]


def update_chart(chart: Dict, release: Dict) -> None:
    chart["version"] = release["operator"]
    chart["appVersion"] = release["operator"]

    for dependency in chart.get("dependencies", []):
        if dependency["name"] == "community-operator-crds":
            dependency["version"] = release["operator"]


def main() -> int:
    # Updating local files
    update_and_write_file(RELATIVE_PATH_TO_MANAGER_YAML, update_operator_deployment)
    update_and_write_file(
        RELATIVE_PATH_TO_OPENSHIFT_MANAGER_YAML, update_operator_deployment
    )

    # Updating Helm Chart files
    update_and_write_file(RELATIVE_PATH_TO_CHART_VALUES, update_chart_values)
    update_and_write_file(RELATIVE_PATH_TO_CHART, update_chart)
    update_and_write_file(RELATIVE_PATH_TO_CRD_CHART, update_chart)

    return 0


if __name__ == "__main__":
    sys.exit(main())
