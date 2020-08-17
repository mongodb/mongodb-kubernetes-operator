import dockerutil
import json
import sys
import argparse


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--old_repo_url", help="Url of the image to retag", type=str)
    parser.add_argument(
        "--new_repo_url", help="Url to where the new image should be pushed", type=str
    )
    parser.add_argument(
        "--path",
        help="path to use for the temporarily generated Dockerfile",
        type=str,
        default=".",
    )
    parser.add_argument("--release_file", help="Path to the release file", type=str)
    parser.add_argument(
        "--old_tag", help="the old tag of the image to retag", type=str,
    )
    parser.add_argument(
        "--username", help="username for the registry", type=str,
    )
    parser.add_argument(
        "--password", help="password for the registry", type=str,
    )
    parser.add_argument(
        "--registry", help="The docker registry", type=str,
    )
    parser.add_argument(
        "--labels", help="Labels for the new image", type=json.loads,
    )
    parser.add_argument("--image_type", help="Type of image to be released")
    args = parser.parse_args()

    return args


def main() -> int:
    args = parse_args()
    with open(args.release_file) as f:
        release = json.load(f)

    if args.image_type == "operator":
        new_tag = release["mongodb-kubernetes-operator"]
    elif args.image_type == "versionhook":
        new_tag = release["version-upgrade-hook"]
    dockerutil.retag_image(
        args.old_repo_url,
        args.new_repo_url,
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
