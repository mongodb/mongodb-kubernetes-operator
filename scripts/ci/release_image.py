import sys
import argparse

sys.path.append("./scripts/dev/")
import dockerutil
import docker
import json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("old_repo_url", help="Url of the image to retag", type=str)
    parser.add_argument(
        "new_repo_url", help="Url to where the new image should be pushed", type=str
    )
    parser.add_argument(
        "--path",
        help="path to use for the temporarily generated Dockerfile",
        type=str,
        default=".",
    )
    parser.add_argument("release_file", help="Path to the release file", type=str)
    parser.add_argument(
        "old_tag", help="the old tag of the image to retag", type=str,
    )
    parser.add_argument(
        "username", help="username for the registry", type=str,
    )
    parser.add_argument(
        "password", help="password for the registry", type=str,
    )
    parser.add_argument(
        "registry", help="The docker registry", type=str,
    )
    parser.add_argument(
        "--labels", help="Labels for the new image", type=json.loads,
    )
    args = parser.parse_args()

    return args


def main() -> int:
    args = parse_args()
    with open(args.release_file) as f:
        release = json.load(f)

    new_tag = release["mongodb-kubernetes-operator"]
    dockerutil.retag_image(
        args.repo_url,
        args.old_tag,
        new_tag,
        args.path,
        args.labels,
        args.username,
        args.password,
        args.registry,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
