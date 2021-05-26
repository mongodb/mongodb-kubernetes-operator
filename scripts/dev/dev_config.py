from __future__ import annotations

import json
from typing import Dict, Optional
from enum import Enum
import os
from dataclasses import dataclass
from dataclasses_json import dataclass_json

CONFIG_PATH = "~/.community-operator-dev/config.json"
FULL_CONFIG_PATH = os.path.expanduser(CONFIG_PATH)
REQUIRED_CONFIG_KEYS = frozenset(
    [
        "repo_url",
        "operator_image",
        "e2e_image",
        "readiness_probe_image",
        "version_upgrade_hook_image",
        "agent_image_ubi",
        "agent_image_ubuntu",
    ]
)


class Distro(Enum):
    UBUNTU = 0
    UBI = 1

    @staticmethod
    def from_string(distro_name: str) -> Distro:
        distro_name = distro_name.lower()
        return {
            "ubuntu": Distro.UBUNTU,
            "ubi": Distro.UBI,
        }[distro_name]


def get_config_path() -> str:
    return os.getenv("MONGODB_COMMUNITY_CONFIG", FULL_CONFIG_PATH)


@dataclass_json
@dataclass
class DevConfig:
    # The namespace that will be used for deploying resources or running tests.
    namespace: str

    # The central repo url where all images will be pushed or pulled.
    repo_url: str

    # The image name of the released operator image.
    operator_image: str

    # The image name that will be used for the e2e test application.
    e2e_image: str

    # The image name of the ubi agent that will be built for development workflow.
    agent_image_ubi_dev: str

    # The image name of the ubuntu agent that will be built for development workflow.
    agent_image_ubuntu_dev: str

    # The image name of the readiness probe image.
    readiness_probe_image: str = ""

    # The image name of the version upgrade hook that will be built for development workflow.
    version_upgrade_hook_image_dev: str = ""

    # The image name of the readiness probe that will be built for development workflow.
    readiness_probe_image_dev: str = ""

    # The image name of the operator that will be built for development workflow.
    operator_image_dev: str = ""

    # The image name of the released agent ubi image.
    agent_image_ubi: str = ""

    # The image name of the released agent ubuntu image.
    agent_image_ubuntu: str = ""

    # The image of the target release destination image for the version upgrade hook
    version_upgrade_hook_image: str = ""

    # optional, required only to run the release process "locally", i.e. publish Dockerfiles to the specified
    # S3 bucket
    s3_bucket: str = ""

    @property
    def role_dir(self) -> str:
        return os.path.join(os.getcwd(), "config", "rbac")

    @property
    def deploy_dir(self) -> str:
        return os.path.join(os.getcwd(), "config", "manager")

    @property
    def test_data_dir(self) -> str:
        return os.path.join(os.getcwd(), "testdata")

    def agent_image(self, distro: Distro) -> str:
        if distro == Distro.UBUNTU:
            return self.agent_image_ubuntu_dev
        return self.agent_image_ubi_dev


def load_config(config_file_path: Optional[str] = None) -> DevConfig:
    if config_file_path is None:
        config_file_path = get_config_path()

    try:
        with open(config_file_path, "r") as f:
            json_config = json.loads(f.read())
            _ensure_dev_images_are_configured(json_config)
            _validate_config(json_config)
            return DevConfig.from_dict(json_config)  # type: ignore
    except FileNotFoundError:
        print(
            f"No DevConfig found. Please ensure that the configuration file exists at '{config_file_path}'"
        )
        raise


def _validate_config(config: Dict[str, str]) -> None:
    """
    _validate_config raises a ValueError if there are any missing keys
    in the given DevConfig.
    """
    missing_keys = REQUIRED_CONFIG_KEYS.difference(config.keys())
    if missing_keys:
        raise ValueError(
            "DevConfig is missing the required keys: [{}]".format(
                ",".join(missing_keys)
            )
        )


def _ensure_dev_images_are_configured(config: Dict[str, str]) -> None:
    """
    _ensure_dev_images_are_configured makes sure that all of the corresponding dev
    images are set. Locally these can be the same as the regular images, but on evergreen
    they can be set to different registries. i.e. not the same registries that the images
    are released to.
    """
    _ensure_dev_image("readiness_probe_image", config)
    _ensure_dev_image("operator_image", config)
    _ensure_dev_image("readiness_probe_image", config)
    _ensure_dev_image("version_upgrade_hook_image", config)
    _ensure_dev_image("agent_image_ubi", config)
    _ensure_dev_image("agent_image_ubuntu", config)


def _ensure_dev_image(image: str, config: Dict[str, str]) -> None:
    """use the same dev registry for dev/regular images when unspecified."""
    if f"{image}_dev" not in config:
        config[f"{image}_dev"] = config[image]
