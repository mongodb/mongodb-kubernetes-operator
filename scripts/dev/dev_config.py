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
    def prehook_image(self):
        return self._config["prehook_image"]

def load_config(config_file_path: str = None) -> Optional[DevConfig]:
    if config_file_path == None:
        config_file_path = get_config_path()
    with open(config_file_path, "r") as f:
        return DevConfig(json.loads(f.read()))

    print(
        f"No DevConfig found. Please ensure that the configuration file exists at '{config_file_path}'"
    )
    return None
