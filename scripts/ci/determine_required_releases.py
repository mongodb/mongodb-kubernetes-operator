#!/usr/bin/env python3

# This script accepts a key from the "release.json" file.
# If the corresponding image of the specified version has been released,

import json
import sys
from typing import List, Dict

import requests

# contains a map of the quay urls to fetch data about the corresponding images.
QUAY_URL_MAP: Dict[str, List[str]] = {
    "agent": [
        "https://quay.io/api/v1/repository/mongodb/mongodb-agent-ubi",
        "https://quay.io/api/v1/repository/mongodb/mongodb-agent",
    ],
    "readiness-probe": [
        "https://quay.io/api/v1/repository/mongodb/mongodb-kubernetes-readinessprobe",
    ],
    "version-upgrade-hook": [
        "https://quay.io/api/v1/repository/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook"
    ],
    "operator": [
        "https://quay.io/api/v1/repository/mongodb/mongodb-kubernetes-operator"
    ],
}


def _get_all_released_tags(url: str) -> List[str]:
    resp = requests.get(url).json()
    tags = resp["tags"]
    return list(tags.keys())


def _load_image_name_to_version_map() -> Dict[str, str]:
    """
    _load_image_name_to_version_map returns a mapping of each image name
    to the corresponding version.

    e.g.
    {
        "mongodb-kubernetes-operator" : "0.7.2",
        "mongodb-agent" : "11.0.11.7036-1"
        ...
    }
    """
    with open("release.json") as f:
        release = json.loads(f.read())

    return release


def _all_urls_are_released(urls: List[str], version: str) -> bool:
    """
    _all_urls_are_released returns True if the given version exists
    as a tag in all urls provided.
    """
    for url in urls:
        tags = _get_all_released_tags(url)
        if version not in tags:
            return False
    return True


def main() -> int:
    if len(sys.argv) != 2:
        raise ValueError("usage: determine_required_releases.py [image-name]")

    image_name = sys.argv[1]
    name_to_version_map = _load_image_name_to_version_map()

    if image_name not in name_to_version_map:
        raise ValueError(
            "Unknown image type [{}], valid values are [{}]".format(
                image_name, ",".join(name_to_version_map.keys())
            )
        )

    if image_name not in QUAY_URL_MAP:
        raise ValueError("No associated image urls for key [{}]".format(image_name))

    if _all_urls_are_released(
        QUAY_URL_MAP[image_name], name_to_version_map[image_name]
    ):
        print("released")
    else:
        print("unreleased")
    return 0


if __name__ == "__main__":
    sys.exit(main())
