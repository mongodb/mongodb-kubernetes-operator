import jinja2
import argparse


def operator_params(adds):
    return {
        "builder": True,
        "builder_image": "golang",
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "adds": adds,
    }


def test_runner_params(adds):
    return {
        "builder": True,
        "builder_image": "golang",  # TODO: make this image smaller. There were errors using alpine
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "adds": adds,
    }


def e2e_params(adds):
    return {
        "base_image": "golang",  # TODO: make this image smaller, error: 'exec: "gcc": executable file not found in $PATH' with golang:alpine
        "adds": adds,
    }


def unit_test_params(adds):
    return {
        "base_image": "golang",
        "adds": adds,
    }


def python_formatting_params(adds, script):
    return {
        "base_image": "python:slim",
        "adds": adds,
        "script_location": script,
    }


def render(image_name, adds, script_location):
    param_dict = {
        "unittest": unit_test_params(adds),
        "e2e": e2e_params(adds),
        "testrunner": test_runner_params(adds),
        "operator": operator_params(adds),
        "python_formatting": python_formatting_params(adds, script_location),
    }

    render_values = param_dict.get(image_name, dict())

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath="scripts/dev/templates")
    return env.get_template("Dockerfile.{}".format(image_name)).render(render_values)


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("image", help="Type of image for the Dockerfile")
    parser.add_argument(
        "--adds",
        help='Paths to use in the ADD command (defaults to ".")',
        type=str,
        default=".",
    )
    parser.add_argument(
        "--script_location",
        help="Location of the python script to run. (Used only for python_formatting)",
        default="./scripts/ci/run_black_formatting_check.sh",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    print(render(args.image, args.adds.split("\n"), args.script_location))


if __name__ == "__main__":
    main()
