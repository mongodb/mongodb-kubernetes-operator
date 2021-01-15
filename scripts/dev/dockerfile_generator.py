import jinja2
import argparse
import os
import sys
from typing import List, Dict, Union

DockerParameters = Dict[str, Union[bool, str, List[str]]]

GOLANG_TAG = "1.14"


def operator_params(files_to_add: List[str]) -> DockerParameters:
    return {
        "builder": True,
        "builder_image": f"golang:{GOLANG_TAG}",
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "files_to_add": files_to_add,
    }


def e2e_params(files_to_add: List[str]) -> DockerParameters:
    return {
        "base_image": f"golang:{GOLANG_TAG}",  # TODO: make this image smaller, error: 'exec: "gcc": executable file not found in $PATH' with golang:alpine
        "files_to_add": files_to_add,
    }


def unit_test_params(files_to_add: List[str]) -> DockerParameters:
    return {
        "base_image": f"golang:{GOLANG_TAG}",
        "files_to_add": files_to_add,
    }


def python_formatting_params(files_to_add: List[str], script: str) -> DockerParameters:
    return {
        "base_image": "python:slim",
        "files_to_add": files_to_add,
        "script_location": script,
    }


def render(image_name: str, files_to_add: List[str], script_location: str) -> str:
    param_dict = {
        "unittest": unit_test_params(files_to_add),
        "e2e": e2e_params(files_to_add),
        "operator": operator_params(files_to_add),
        "python_formatting": python_formatting_params(files_to_add, script_location),
    }

    render_values = param_dict.get(image_name, dict())

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath="scripts/dev/templates")
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
    parser.add_argument(
        "--script_location",
        help="Location of the python script to run. (Used only for python_formatting)",
        type=str,
    )
    args = parser.parse_args()

    if args.script_location is not None and args.image != "python_formatting":
        parser.error(
            'The --script_location argument can be used only when the image used is "python_formatting"'
        )

    return args


def main() -> int:
    args = parse_args()
    print(render(args.image, args.files_to_add.split(os.linesep), args.script_location))
    return 0


if __name__ == "__main__":
    sys.exit(main())
