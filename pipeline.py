import argparse
import json
import subprocess
import sys
from typing import Dict, List, Set
from scripts.ci.base_logger import logger
from scripts.ci.images_signing import (
    sign_image,
    verify_signature,
    mongodb_artifactory_login,
)

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

AGENT_DISTRO_KEY = "agent_distro"
TOOLS_DISTRO_KEY = "tools_distro"

AGENT_DISTROS_PER_ARCH = {
    "amd64": {AGENT_DISTRO_KEY: "rhel8_x86_64", TOOLS_DISTRO_KEY: "rhel88-x86_64"},
    "arm64": {AGENT_DISTRO_KEY: "amzn2_aarch64", TOOLS_DISTRO_KEY: "rhel88-aarch64"},
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
        "skip_tags": config.skip_tags,  # Include skip_tags
        "include_tags": config.include_tags,  # Include include_tags
    }

    # Handle special cases
    if image_name == "operator":
        arguments["inventory"] = "inventories/operator-inventory.yaml"

    if image_name == "e2e":
        arguments.pop("builder", None)
        arguments["base_image"] = release["golang-builder-image"]
        arguments["inventory"] = "inventories/e2e-inventory.yaml"

    if image_name == "agent":
        arguments["tools_version"] = release["agent-tools-version"]

    return arguments


def sign_and_verify(registry: str, tag: str) -> None:
    sign_image(registry, tag)
    verify_signature(registry, tag)


def build_and_push_image(
    image_name: str,
    config: DevConfig,
    args: Dict[str, str],
    architectures: Set[str],
    release: bool,
    sign: bool,
    insecure: bool = False,
) -> None:
    if sign:
        mongodb_artifactory_login()
    for arch in architectures:
        image_tag = f"{image_name}"
        args["architecture"] = arch
        if image_name == "agent":
            args[AGENT_DISTRO_KEY] = AGENT_DISTROS_PER_ARCH[arch][AGENT_DISTRO_KEY]
            args[TOOLS_DISTRO_KEY] = AGENT_DISTROS_PER_ARCH[arch][TOOLS_DISTRO_KEY]
        process_image(
            image_tag,
            build_args=args,
            inventory=args["inventory"],
            skip_tags=args["skip_tags"],
            include_tags=args["include_tags"],
        )
        if release:
            registry = args["registry"] + "/" + args["image"]
            context_tag = args["release_version"] + "-context-" + arch
            release_tag = args["release_version"] + "-" + arch
            if sign:
                sign_and_verify(registry, context_tag)
                sign_and_verify(registry, release_tag)

    if args["image_dev"]:
        image_to_push = args["image_dev"]
    elif image_name == "e2e":
        # If no image dev (only e2e is concerned) we push the normal image
        image_to_push = args["image"]
    else:
        raise Exception("Dev image must be specified")

    push_manifest(config, architectures, image_to_push, insecure)

    if config.gh_run_id:
        push_manifest(config, architectures, image_to_push, insecure, config.gh_run_id)

    if release:
        registry = args["registry"] + "/" + args["image"]
        context_tag = args["release_version"] + "-context"
        push_manifest(
            config, architectures, args["image"], insecure, args["release_version"]
        )
        push_manifest(config, architectures, args["image"], insecure, context_tag)
        if sign:
            sign_and_verify(registry, args["release_version"])
            sign_and_verify(registry, context_tag)


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
    insecure: bool = False,
    image_tag: str = "latest",
) -> None:
    logger.info(f"Pushing manifest for {image_tag}")
    final_manifest = "{0}/{1}:{2}".format(config.repo_url, image_name, image_tag)
    remove_args = ["docker", "manifest", "rm", final_manifest]
    logger.info("Removing existing manifest")
    run_cli_command(remove_args, fail_on_error=False)

    create_args = [
        "docker",
        "manifest",
        "create",
        final_manifest,
    ]

    if insecure:
        create_args.append("--insecure")

    for arch in architectures:
        create_args.extend(["--amend", final_manifest + "-" + arch])

    logger.info("Creating new manifest")
    run_cli_command(create_args)

    push_args = ["docker", "manifest", "push", final_manifest]
    logger.info("Pushing new manifest")
    run_cli_command(push_args)


# Raises exceptions by default
def run_cli_command(args: List[str], fail_on_error: bool = True) -> None:
    command = " ".join(args)
    logger.debug(f"Running: {command}")
    try:
        cp = subprocess.run(
            command,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=True,
            check=False,
        )
    except Exception as e:
        logger.error(f" Command raised the following exception: {e}")
        if fail_on_error:
            raise Exception
        else:
            logger.warning("Continuing...")
            return

    if cp.returncode != 0:
        error_msg = cp.stderr.decode().strip()
        stdout = cp.stdout.decode().strip()
        logger.error(f"Error running command")
        logger.error(f"stdout:\n{stdout}")
        logger.error(f"stderr:\n{error_msg}")
        if fail_on_error:
            raise Exception
        else:
            logger.warning("Continuing...")
            return


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--image-name", type=str)
    parser.add_argument("--release", action="store_true", default=False)
    parser.add_argument(
        "--arch",
        choices=["amd64", "arm64"],
        nargs="+",
        help="for daily builds only, specify the list of architectures to build for images",
    )
    parser.add_argument("--tag", type=str)
    parser.add_argument("--sign", action="store_true", default=False)
    parser.add_argument("--insecure", action="store_true", default=False)
    return parser.parse_args()


"""
Takes arguments:
--image-name : The name of the image to build, must be one of VALID_IMAGE_NAMES
--release : We push the image to the registry only if this flag is set
--architecture : List of architectures to build for the image
--sign : Sign images with our private key if sign is set (only for release)

Run with --help for more information
Example usage : `python pipeline.py --image-name agent --release --sign`

Builds and push the docker image to the registry
Many parameters are defined in the dev configuration, default path is : ~/.community-operator-dev/config.json
"""


def main() -> int:
    args = _parse_args()

    image_name = args.image_name
    if image_name not in VALID_IMAGE_NAMES:
        logger.error(
            f"Invalid image name: {image_name}. Valid options are: {VALID_IMAGE_NAMES}"
        )
        return 1

    # Handle dev config
    config: DevConfig = load_config()
    config.gh_run_id = args.tag

    # Warn user if trying to release E2E tests
    if args.release and image_name == "e2e":
        logger.warning(
            "Warning : releasing E2E test will fail because E2E image has no release version"
        )

    # Skipping release tasks by default
    if not args.release:
        config.ensure_skip_tag("release")
        if args.sign:
            logger.warning("--sign flag has no effect without --release")

    if args.arch:
        arch_set = set(args.arch)
    else:
        # Default is multi-arch
        arch_set = {"amd64", "arm64"}
    logger.info(f"Building for architectures: {','.join(arch_set)}")

    if not args.sign:
        logger.warning("--sign flag not provided, images won't be signed")

    image_args = build_image_args(config, image_name)

    build_and_push_image(
        image_name, config, image_args, arch_set, args.release, args.sign, args.insecure
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
