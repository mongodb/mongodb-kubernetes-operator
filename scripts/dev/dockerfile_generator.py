import jinja2
import argparse


def operator_params():
    return {
        "builder": True,
        "builder_image": "golang",
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
    }


def test_runner_params():
    return {
        "builder": True,
        "builder_image": "golang",  # TODO: make this image smaller. There were errors using alpine
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
    }


def e2e_params():
    return {
        "base_image": "golang",  # TODO: make this image smaller, error: 'exec: "gcc": executable file not found in $PATH' with golang:alpine
    }


def unit_test_params():
    return {
        "base_image": "golang",
    }


def linting_check_params():
    return {
        "base_image": "python:alpine",
    }


def render(image_name):
    param_dict = {
        "unittest": unit_test_params(),
        "e2e": e2e_params(),
        "testrunner": test_runner_params(),
        "operator": operator_params(),
        "linting": linting_check_params(),
    }

    render_values = param_dict.get(image_name, dict())

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath="scripts/dev/templates")
    return env.get_template("Dockerfile.{}".format(image_name)).render(render_values)


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("image", help="Type of image for the Dockerfile")
    return parser.parse_args()


def main():
    args = parse_args()
    print(render(args.image))


if __name__ == "__main__":
    main()
