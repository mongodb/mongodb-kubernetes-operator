import jinja2
import argparse
import os


def operator_params(files_to_add):
    return {
        "builder": True,
        "builder_image": "golang",
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "files_to_add": files_to_add,
    }


def test_runner_params(files_to_add):
    return {
        "builder": True,
        "builder_image": "golang",  # TODO: make this image smaller. There were errors using alpine
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "files_to_add": files_to_add,
    }


def e2e_params(files_to_add):
    return {
        "base_image": "golang",  # TODO: make this image smaller, error: 'exec: "gcc": executable file not found in $PATH' with golang:alpine
        "files_to_add": files_to_add,
    }


def unit_test_params(files_to_add):
    return {
        "base_image": "golang",
        "files_to_add": files_to_add,
    }


def python_formatting_params(files_to_add, script):
    return {
        "base_image": "python:slim",
        "files_to_add": files_to_add,
        "script_location": script,
    }


def render(image_name, files_to_add, script_location):
    param_dict = {
        "unittest": unit_test_params(files_to_add),
        "e2e": e2e_params(files_to_add),
        "testrunner": test_runner_params(files_to_add),
        "operator": operator_params(files_to_add),
        "python_formatting": python_formatting_params(files_to_add, script_location),
    }

    render_values = param_dict.get(image_name, dict())

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath="scripts/dev/templates")
    return env.get_template(f"Dockerfile.{image_name}").render(render_values)


def parse_args():
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


def main():
    args = parse_args()
    print(render(args.image, args.files_to_add.split(os.linesep), args.script_location))


if __name__ == "__main__":
    main()
