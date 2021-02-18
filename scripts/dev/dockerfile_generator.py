import json

import jinja2
import argparse
import os
import sys
from typing import List, Dict, Union

DockerParameters = Dict[str, Union[bool, str, List[str]]]

GOLANG_TAG = "1.15"


def operator_params(files_to_add: List[str]) -> DockerParameters:
    return {
        "builder": True,
        "builder_image": f"golang:{GOLANG_TAG}",
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "files_to_add": files_to_add,
    }


def e2e_params(files_to_add: List[str]) -> DockerParameters:
    return {
        "base_image": f"golang:{GOLANG_TAG}",
        # TODO: make this image smaller, error: 'exec: "gcc": executable file not found in $PATH' with golang:alpine
        "files_to_add": files_to_add,
    }


def render(image_name: str, files_to_add: List[str]) -> str:
    param_dict = {
        "e2e": e2e_params(files_to_add),
        "operator": operator_params(files_to_add),
    }

    render_values = param_dict.get(image_name, dict())

    search_path = str(render_values.get("template_path", "scripts/dev/templates"))

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath=search_path)
    return env.get_template(f"Dockerfile.{image_name}").render(render_values)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("image", help="Type of image for the Dockerfile")
    parser.add_argument(
        "--files_to_add",
        help='Paths to use in the ADD command (defaults to ".")',
        type=str,
        default=".",
    )

    return parser.parse_args()


def main() -> int:
    args = parse_args()
    print(render(args.image, args.files_to_add.split(os.linesep)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
