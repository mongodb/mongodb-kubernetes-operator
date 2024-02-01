from __future__ import annotations
from typing import Dict, Optional, List
from enum import Enum
import json
import os

CONFIG_PATH = "~/.community-operator-dev/config.json"
FULL_CONFIG_PATH = os.path.expanduser(CONFIG_PATH)


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


class DevConfig:
    """
    DevConfig is a wrapper around the developer configuration file
    """

    def __init__(self, config: Dict, distro: Distro):
        self._config = config
        self._distro = distro
        self.include_tags: List[str] = []
        self.skip_tags: List[str] = []
        self.gh_run_id = ""

    def ensure_tag_is_run(self, tag: str) -> None:
        if tag not in self.include_tags:
            self.include_tags.append(tag)
        if tag in self.skip_tags:
            self.skip_tags.remove(tag)

    @property
    def namespace(self) -> str:
        return self._config["namespace"]

    @property
    def repo_url(self) -> str:
        return self._config["repo_url"]

    @property
    def s3_bucket(self) -> str:
        return self._config["s3_bucket"]

    @property
    def expire_after(self) -> str:
        return self._config.get("expire_after", "never")

    @property
    def operator_image(self) -> str:
        return self._config["operator_image"]

    @property
    def operator_image_dev(self) -> str:
        return self._get_dev_image("operator_image_dev", "operator_image")

    @property
    def e2e_image(self) -> str:
        return self._config["e2e_image"]

    @property
    def version_upgrade_hook_image(self) -> str:
        return self._config["version_upgrade_hook_image"]

    @property
    def version_upgrade_hook_image_dev(self) -> str:
        return self._get_dev_image(
            "version_upgrade_hook_image_dev", "version_upgrade_hook_image"
        )

    @property
    def readiness_probe_image(self) -> str:
        return self._config["readiness_probe_image"]

    # these directories are used from within the E2E tests when running locally.
    @property
    def role_dir(self) -> str:
        if "role_dir" in self._config:
            return self._config["role_dir"]
        return os.path.join(os.getcwd(), "config", "rbac")

    @property
    def deploy_dir(self) -> str:
        if "deploy_dir" in self._config:
            return self._config["deploy_dir"]
        return os.path.join(os.getcwd(), "config", "manager")

    @property
    def test_data_dir(self) -> str:
        if "test_data_dir" in self._config:
            return self._config["test_data_dir"]
        return os.path.join(os.getcwd(), "testdata")

    @property
    def readiness_probe_image_dev(self) -> str:
        return self._get_dev_image("readiness_probe_image_dev", "readiness_probe_image")

    @property
    def mongodb_image_name(self) -> str:
        return self._config.get("mongodb_image_name", "mongodb-community-server")

    @property
    def mongodb_image_repo_url(self) -> str:
        return self._config.get("mongodb_image_repo_url", "quay.io/mongodb")

    @property
    def agent_image(self) -> str:
        return self._config["agent_image"]

    @property
    def local_operator(self) -> str:
        return self._config["mdb_local_operator"]

    @property
    def kube_config(self) -> str:
        return self._config["kubeconfig"]

    @property
    def agent_image_dev(self) -> str:
        return self._get_dev_image("agent_image_dev", "agent_image")

    @property
    def image_type(self) -> str:
        if self._distro == Distro.UBI:
            return "ubi8"
        return "ubuntu-2004"

    def ensure_skip_tag(self, tag: str) -> None:
        if tag not in self.skip_tags:
            self.skip_tags.append(tag)

    def _get_dev_image(self, dev_image: str, image: str) -> str:
        if dev_image in self._config:
            return self._config[dev_image]
        return self._config[image]


def load_config(
    config_file_path: Optional[str] = None, distro: Distro = Distro.UBI
) -> DevConfig:
    if config_file_path is None:
        config_file_path = get_config_path()

    try:
        with open(config_file_path, "r") as f:
            return DevConfig(json.loads(f.read()), distro=distro)
    except FileNotFoundError:
        print(
            f"No DevConfig found. Please ensure that the configuration file exists at '{config_file_path}'"
        )
        raise
