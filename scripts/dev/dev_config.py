from typing import Dict, Optional
import json
import os

CONFIG_PATH = "~/.community-operator-dev/config.json"
FULL_CONFIG_PATH = os.path.expanduser(CONFIG_PATH)


def get_config_path() -> str:
    return os.getenv("MONGODB_COMMUNITY_CONFIG", FULL_CONFIG_PATH)


class DevConfig:
    """
    DevConfig is a wrapper around the developer configuration file
    """

    def __init__(self, config):
        self._config = config

    @property
    def namespace(self):
        return self._config["namespace"]

    @property
    def repo_url(self):
        return self._config["repo_url"]

    @property
    def operator_image(self):
        return self._config["operator_image"]

    @property
    def e2e_image(self):
        return self._config["e2e_image"]

    @property
    def prestop_hook_image(self):
        return self._config["prestop_hook_image"]

    @property
    def testrunner_image(self):
        return self._config["testrunner_image"]


def load_config(config_file_path: str = None) -> Optional[DevConfig]:
    print("Config file path: {}".format(config_file_path))
    if config_file_path == None:
        config_file_path = get_config_path()
    with open(config_file_path, "r") as f:
        return DevConfig(json.loads(f.read()))

    print(
        "No DevConfig found. Please ensure that the configuration file exists at '{}'".format(config_file_path)
    )
    return None
