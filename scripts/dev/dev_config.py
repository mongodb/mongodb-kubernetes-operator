from typing import Dict, Optional
import json
import os

CONFIG_PATH = "~/.community-operator-dev/config.json"
FULL_CONFIG_PATH = os.path.expanduser(CONFIG_PATH)


class DevConfig:
    def __init__(self, config):
        self._config = config

    @property
    def namespace(self):
        return self._config["namespace"]

    @property
    def repo_url(self):
        return self._config["repo_url"]


def load_config() -> Optional[DevConfig]:
    with open(FULL_CONFIG_PATH, "r") as f:
        return DevConfig(json.loads(f.read()))

    print(
        f"No DevConfig found. Please ensure that the configuration file exists at '{FULL_CONFIG_PATH}'"
    )
    return None
