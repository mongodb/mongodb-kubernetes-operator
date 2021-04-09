import argparse
import json
import sys
from typing import Dict

from sonar.sonar import process_image

from scripts.dev.dev_config import load_config, DevConfig

VALID_IMAGE_NAMES = frozenset(["agent-ubi", "agent-ubuntu"])

DEFAULT_IMAGE_TYPE = "ubuntu"
DEFAULT_NAMESPACE = "default"


def build_agent_image_ubi(config: DevConfig, labels: Dict[str, str]):
    image_name = "agent-ubi"
    with open("release.json") as f:
        release = json.loads(f.read())
    args = {
        "agent_version": release["agent"]["version"],
        "tools_version": release["agent"]["tools_version"],
        "tools_distro": "ubuntu1604-x86_64",
        "agent_distro": "linux_x86_64",
        "registry": config.repo_url,
    }
    sonar_build_image(
        image_name,
        args=args,
        labels=labels,
    )


def build_agent_image_ubuntu(config: DevConfig, labels: Dict[str, str]):
    image_name = "agent-ubuntu"
    with open("release.json") as f:
        release = json.loads(f.read())
    args = {
        "agent_version": release["agent"]["version"],
        "tools_version": release["agent"]["tools_version"],
        "tools_distro": "rhel70-x86_64",
        "agent_distro": "rhel7_x86_64",
        "registry": config.repo_url,
    }
    sonar_build_image(
        image_name,
        args=args,
        labels=labels,
    )


def sonar_build_image(
    image_name: str,
    args: Dict[str, str] = None,
    inventory="inventory.yaml",
    labels: Dict[str, str] = None,
):
    """Calls sonar to build `image_name` with arguments defined in `args`."""
    process_image(
        image_name,
        build_args=args,
        inventory=inventory,
        include_tags=[],
        skip_tags=[],
        labels=labels,
    )


def _parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image-name", type=str)
    parser.add_argument("--labels", type=str)
    return parser.parse_args()


def main() -> int:
    args = _parse_args()

    image_name = args.image_name
    if image_name not in VALID_IMAGE_NAMES:
        print(
            f"Image name [{image_name}] is not valid. Must be one of [{', '.join(VALID_IMAGE_NAMES)}]"
        )
        return 1

    agent_build_function = {
        "agent-ubi": build_agent_image_ubi,
        "agent-ubuntu": build_agent_image_ubuntu,
    }[image_name]

    labels = []
    if args.labels:
        labels = args.labels.split(",")

    labels_dict = {}
    for label in labels:
        key, val = label.split("=")
        labels_dict[key] = val

    agent_build_function(load_config(), labels=labels)
    return 0


if __name__ == "__main__":
    sys.exit(main())
