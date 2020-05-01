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


def render(image_name):
    param_dict = {
        "unittest": unit_test_params(),
        "e2e": e2e_params(),
        "testrunner": test_runner_params(),
        "operator": operator_params(),
    }

    if image_name not in param_dict:
        raise ValueError(
            "Image name: {} is invalid. Valid values are {}".format(
                image_name, param_dict.keys()
            )
        )

    env = jinja2.Environment()
    env.loader = jinja2.FileSystemLoader(searchpath="scripts/dev/templates")
    return env.get_template("Dockerfile.{}".format(image_name)).render(
        param_dict[image_name]
    )


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("image", help="Type of image for the Dockerfile")
    return parser.parse_args()


def main():
    args = parse_args()
    print(render(args.image))


if __name__ == "__main__":
    main()
