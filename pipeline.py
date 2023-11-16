import argparse
import json
import subprocess
import sys
from typing import Dict, List, Set

from scripts.dev.dev_config import load_config, DevConfig
from sonar.sonar import process_image

# These image names must correspond to prefixes in release.json, developer configuration and inventories
VALID_IMAGE_NAMES = {
    "agent",
    "readiness-probe",
    "version-upgrade-hook",
    "operator",
    "e2e",
}


def load_release() -> Dict:
    with open("release.json") as f:
        return json.load(f)


def build_image_args(config: DevConfig, image_name: str) -> Dict[str, str]:
    release = load_release()

    # Naming in pipeline : readiness-probe, naming in dev config : readiness_probe_image
    image_name_prefix = image_name.replace("-", "_")

    # Default config
    arguments = {
        "builder": "true",
        # Defaults to "" if empty, e2e has no release version
        "release_version": release.get(image_name, ""),
        "tools_version": "",
        "image": getattr(config, f"{image_name_prefix}_image"),
        # Defaults to "" if empty, e2e has no dev image
        "image_dev": getattr(config, f"{image_name_prefix}_image_dev", ""),
        "registry": config.repo_url,
        "s3_bucket": config.s3_bucket,
        "builder_image": release["golang-builder-image"],
        "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
        "inventory": "inventory.yaml",
        # These two options below can probably be removed
        "skip_tags": config.skip_tags,  # Include skip_tags
        "include_tags": config.include_tags,  # Include include_tags
    }

    # Handle special cases
    if image_name == "operator":
        arguments["inventory"] = "inventories/operator-inventory.yaml"

    if image_name == "e2e":
        arguments["base_image"] = release["golang-builder-image"]
        arguments["inventory"] = "inventories/e2e-inventory.yaml"

    if image_name == "agent":
        arguments["tools_version"] = release["agent-tools-version"]

    return arguments


def build_and_push_image(
    image_name: str,
    config: DevConfig,
    args: Dict[str, str],
    architectures: Set[str],
    release: bool,
):
    for arch in architectures:
        image_tag = f"{image_name}-{arch}"
        process_image(
            image_tag,
            build_args=args,
            inventory=args["inventory"],
            skip_tags=args["skip_tags"],
            include_tags=args["include_tags"],
        )
    if release:
        # TODO : is the release with gh_run_id still needed ?
        push_manifest(config, architectures, args["image_dev"])
        push_manifest(config, architectures, args["image"], args["release_version"])
        push_manifest(
            config, architectures, args["image"], args["release_version"] + "-context"
        )


"""
Generates docker manifests by running the following commands:
1. Clear existing manifests
docker manifest rm config.repo_url/image:tag
2. Create the manifest
docker manifest create config.repo_url/image:tag --amend config.repo_url/image:tag-amd64 --amend config.repo_url/image:tag-arm64
3. Push the manifest
docker manifest push config.repo_url/image:tag
"""


def push_manifest(
    config: DevConfig,
    architectures: Set[str],
    image_name: str,
    image_tag: str = "latest",
):
    print(f"Pushing manifest for {image_tag}")
    final_manifest = "{0}/{1}:{2}".format(config.repo_url, image_name, image_tag)
    remove_args = ["docker", "manifest", "rm", final_manifest]
    print("Removing existing manifest")
    run_cli_command(remove_args, raise_exception=False)

    create_args = [
        "docker",
        "manifest",
        "create",
        final_manifest,
    ]

    for arch in architectures:
        create_args.extend(["--amend", final_manifest + "-" + arch])

    print("Creating new manifest")
    run_cli_command(create_args)

    push_args = ["docker", "manifest", "push", final_manifest]
    print("Pushing new manifest")
    run_cli_command(push_args)


# Raises exceptions by default but this can be deactivated
def run_cli_command(args: List[str], raise_exception: bool = True):
    command = " ".join(args)
    print(f"Running: {command}")
    try:
        cp = subprocess.run(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=True,
            check=False,
        )
    except Exception as e:
        print(f"Error executing command: {e}")
        if raise_exception:
            raise Exception
        else:
            print("Continuing...")
            return

    if cp.returncode != 0:
        error_msg = cp.stderr.decode().strip()
        stdout = cp.stdout.decode().strip()
        print(f"Error running command")
        print(f"stdout:\n{stdout}")
        print(f"stderr:\n{error_msg}")
        if raise_exception:
            raise Exception
        else:
            print("Continuing...")
            return


"""
Takes arguments:
--image-name : The name of the image to build, must be one of VALID_IMAGE_NAMES
--release : We push the image to the registry only if this flag is set
--architecture : List of architectures to build for the image

Run with --help for more information
Example usage : `python pipeline.py --image-name agent --release`

Builds and push the docker image to the registry
Many parameters are defined in the dev configuration, default path is : ~/.community-operator-dev/config.json
"""


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--image-name", type=str)
    parser.add_argument("--release", action="store_true", default=False)
    parser.add_argument(
        "--arch",
        choices=["amd64", "arm64"],
        nargs="+",
        help="for daily builds only, specify the list of architectures to build for images",
    )
    args = parser.parse_args()
    image_name = args.image_name
    config: DevConfig = load_config()

    if args.arch:
        arch_set = set(args.arch)
    else:
        # Default is multi-arch
        arch_set = ["amd64", "arm64"]
    print("Building for architectures:", ", ".join(map(str, arch_set)))

    if image_name not in VALID_IMAGE_NAMES:
        print(
            f"Invalid image name: {image_name}. Valid options are: {VALID_IMAGE_NAMES}"
        )
        return 1

    image_args = build_image_args(config, image_name)

    build_and_push_image(image_name, config, image_args, arch_set, args.release)
    return 0


if __name__ == "__main__":
    sys.exit(main())
