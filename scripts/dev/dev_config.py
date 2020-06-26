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


def load_config() -> Optional[DevConfig]:
    config_file_path = get_config_path()
    with open(config_file_path, "r") as f:
        return DevConfig(json.loads(f.read()))




    print(
        f"No DevConfig found. Please ensure that the configuration file exists at '{config_file_path}'"
    )
    return None
