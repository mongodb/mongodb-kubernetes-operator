#!/usr/bin/env python

import json
import sys
import os
from typing import Dict


def _load_release() -> Dict:
    with open("release.json") as f:
        release = json.loads(f.read())
    return release


def _open_url(url: str) -> None:
    os.system("open '" + url + "'")


def main() -> int:
    release_version = _load_release()["mongodb-kubernetes-operator"]
    _open_url(
        f"https://jira.mongodb.org/issues?jql=project%20%3D%20%22Cloud%20Services%22%20%20AND%20component%20%3D%20%22Kubernetes%20Community%22%20%20And%20fixVersion%20%3D%20kube-community-${release_version}%20AND%20status%20in%20(closed%2C%20resolved)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
