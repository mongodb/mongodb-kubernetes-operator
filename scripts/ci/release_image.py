import sys
import argparse

sys.path.append("./scripts/dev/")
import dockerutil
import json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("repo_url", help="Url of the image to retag", type=str)
    parser.add_argument(
        "--path",
        help="path to use for the temporarily generated Dockerfile",
        type=str,
        default=".",
    )
    parser.add_argument(
        "old_tag", help="the old tag of the image to retag", type=str,
    )
    parser.add_argument(
        "new_tag", help="the new tag of the image to retag", type=str,
    )
    parser.add_argument(
        "--labels", help="Labels for the new image", type=json.loads,
    )
    args = parser.parse_args()

    return args


def main() -> int:
    args = parse_args()
    dockerutil.retag_image(
        args.repo_url, args.old_tag, args.new_tag, args.path, args.labels
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
