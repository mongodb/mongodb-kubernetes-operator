import argparse
import json
import sys
import subprocess
from typing import Dict, Optional

from sonar.sonar import process_image

from scripts.dev.dev_config import load_config, DevConfig

VALID_IMAGE_NAMES = frozenset(
    [
        "agent-ubi",
        "agent-ubuntu",
        "readiness-probe-init",
        "version-post-start-hook-init",
        "operator-ubi",
        "e2e",
    ]
)

DEFAULT_IMAGE_TYPE = "ubi"
DEFAULT_NAMESPACE = "default"


def _load_release() -> Dict:
    with open("release.json") as f:
        release = json.loads(f.read())
    return release


def _build_agent_args(config: DevConfig) -> Dict[str, str]:
    release = _load_release()
    return {
        "agent_version": release["mongodb-agent"]["version"],
        "release_version": release["mongodb-agent"]["version"],
        "tools_version": release["mongodb-agent"]["tools_version"],
        "agent_image": config.agent_image,
        "agent_image_dev": config.agent_dev_image,
        "registry": config.repo_url,
        "s3_bucket": config.s3_bucket,
    }


def build_agent_image_ubi(config: DevConfig) -> None:
    image_name = "agent-ubi"
    args = _build_agent_args(config)
    args["agent_image"] = config.agent_image_ubi
    args["agent_image_dev"] = config.agent_dev_image_ubi
    config.ensure_tag_is_run("ubi")

    sonar_build_image(
        image_name,
        config,
        args=args,
    )


def build_agent_image_ubuntu(config: DevConfig) -> None:
    image_name = "agent-ubuntu"
    args = _build_agent_args(config)
    args["agent_image"] = config.agent_image_ubuntu
    args["agent_image_dev"] = config.agent_dev_image_ubuntu
    config.ensure_tag_is_run("ubuntu")

    sonar_build_image(
        image_name,
        config,
        args=args,
    )


def build_readiness_probe_image(config: DevConfig) -> None:
    release = _load_release()
    config.ensure_tag_is_run("readiness-probe")
    config.ensure_tag_is_run("ubi")

    sonar_build_image(
        "readiness-probe-init-amd64",
        config,
        args={
            "builder": "true",
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "registry": config.repo_url,
            "release_version": release["readiness-probe"],
            "readiness_probe_image": config.readiness_probe_image,
            "readiness_probe_image_dev": config.readiness_probe_image_dev,
            "builder_image": release["golang-builder-image"],
            "s3_bucket": config.s3_bucket,
        },
    )

    sonar_build_image(
        "readiness-probe-init-arm64",
        config,
        args={
            "builder": "true",
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "registry": config.repo_url,
            "release_version": release["readiness-probe"],
            "readiness_probe_image": config.readiness_probe_image,
            "readiness_probe_image_dev": config.readiness_probe_image_dev,
            "builder_image": release["golang-builder-image"],
            "s3_bucket": config.s3_bucket,
        },
    )

    create_and_push_manifest(config, config.readiness_probe_image_dev)

    if "release" in config.include_tags:
        create_and_push_manifest(
            config, config.readiness_probe_image, release["readiness-probe"]
        )


def build_version_post_start_hook_image(config: DevConfig) -> None:
    release = _load_release()
    config.ensure_tag_is_run("post-start-hook")
    config.ensure_tag_is_run("ubi")

    sonar_build_image(
        "version-post-start-hook-init-amd64",
        config,
        args={
            "builder": "true",
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "registry": config.repo_url,
            "release_version": release["version-upgrade-hook"],
            "version_post_start_hook_image": config.version_upgrade_hook_image,
            "version_post_start_hook_image_dev": config.version_upgrade_hook_image_dev,
            "builder_image": release["golang-builder-image"],
            "s3_bucket": config.s3_bucket,
        },
    )

    sonar_build_image(
        "version-post-start-hook-init-arm64",
        config,
        args={
            "builder": "true",
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "registry": config.repo_url,
            "release_version": release["version-upgrade-hook"],
            "version_post_start_hook_image": config.version_upgrade_hook_image,
            "version_post_start_hook_image_dev": config.version_upgrade_hook_image_dev,
            "builder_image": release["golang-builder-image"],
            "s3_bucket": config.s3_bucket,
        },
    )

    create_and_push_manifest(config, config.version_upgrade_hook_image_dev)

    if "release" in config.include_tags:
        create_and_push_manifest(
            config, config.version_upgrade_hook_image, release["version-upgrade-hook"]
        )


def build_operator_ubi_image(config: DevConfig) -> None:
    release = _load_release()
    config.ensure_tag_is_run("ubi")
    sonar_build_image(
        "operator-ubi-amd64",
        config,
        args={
            "registry": config.repo_url,
            "builder": "true",
            "builder_image": release["golang-builder-image"],
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "operator_image": config.operator_image,
            "operator_image_dev": config.operator_image_dev,
            "release_version": release["mongodb-kubernetes-operator"],
            "s3_bucket": config.s3_bucket,
        },
        inventory="inventories/operator-inventory.yaml",
    )
    sonar_build_image(
        "operator-ubi-arm64",
        config,
        args={
            "registry": config.repo_url,
            "builder": "true",
            "builder_image": release["golang-builder-image"],
            "base_image": "registry.access.redhat.com/ubi8/ubi-minimal:latest",
            "operator_image": config.operator_image,
            "operator_image_dev": config.operator_image_dev,
            "release_version": release["mongodb-kubernetes-operator"],
            "s3_bucket": config.s3_bucket,
        },
        inventory="inventories/operator-inventory.yaml",
    )

    create_and_push_manifest(config, config.operator_image_dev)

    if "release" in config.include_tags:
        create_and_push_manifest(
            config, config.operator_image, release["mongodb-kubernetes-operator"]
        )


def build_e2e_image(config: DevConfig) -> None:
    release = _load_release()
    sonar_build_image(
        "e2e-arm64",
        config,
        args={
            "registry": config.repo_url,
            "base_image": release["golang-builder-image"],
            "e2e_image": config.e2e_image,
        },
        inventory="inventories/e2e-inventory.yaml",
    )
    sonar_build_image(
        "e2e-amd64",
        config,
        args={
            "registry": config.repo_url,
            "base_image": release["golang-builder-image"],
            "e2e_image": config.e2e_image,
        },
        inventory="inventories/e2e-inventory.yaml",
    )

    create_and_push_manifest(config, config.e2e_image)


def create_and_push_manifest(
    config: DevConfig, image: str, tag: str = "latest"
) -> None:
    final_manifest = "{0}/{1}:{2}".format(config.repo_url, image, tag)
    args = ["docker", "manifest", "rm", final_manifest]
    args_str = " ".join(args)
    print(f"removing existing manifest: {args_str}")
    subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    args = [
        "docker",
        "manifest",
        "create",
        final_manifest,
        "--amend",
        final_manifest + "-amd64",
        "--amend",
        final_manifest + "-arm64",
    ]
    args_str = " ".join(args)
    print(f"creating new manifest: {args_str}")
    cp = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    if cp.returncode != 0:
        raise Exception(cp.stderr)

    args = ["docker", "manifest", "push", final_manifest]
    args_str = " ".join(args)
    print(f"pushing new manifest: {args_str}")
    cp = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    if cp.returncode != 0:
        raise Exception(cp.stderr)


# docker manifest rm $(REPO_URL)/$(OPERATOR_IMAGE):latest | true
# docker manifest create $(REPO_URL)/$(OPERATOR_IMAGE):latest --amend $(REPO_URL)/$(OPERATOR_IMAGE):latest-amd64 --amend $(REPO_URL)/$(OPERATOR_IMAGE):latest-arm64
# docker manifest push $(REPO_URL)/$(OPERATOR_IMAGE):latest


def sonar_build_image(
    image_name: str,
    config: DevConfig,
    args: Optional[Dict[str, str]] = None,
    inventory: str = "inventory.yaml",
) -> None:
    """Calls sonar to build `image_name` with arguments defined in `args`."""
    process_image(
        image_name,
        build_args=args,
        inventory=inventory,
        include_tags=config.include_tags,
        skip_tags=config.skip_tags,
    )


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--image-name", type=str)
    parser.add_argument("--release", type=lambda x: x.lower() == "true")
    return parser.parse_args()


def main() -> int:
    args = _parse_args()

    image_name = args.image_name
    if image_name not in VALID_IMAGE_NAMES:
        print(
            f"Image name [{image_name}] is not valid. Must be one of [{', '.join(VALID_IMAGE_NAMES)}]"
        )
        return 1

    config = load_config()

    # by default we do not want to run any release tasks. We must explicitly
    # use the --release flag to run them.
    config.ensure_skip_tag("release")

    # specify --release to release the image
    if args.release:
        config.ensure_tag_is_run("release")

    image_build_function = {
        "agent-ubi": build_agent_image_ubi,
        "agent-ubuntu": build_agent_image_ubuntu,
        "readiness-probe-init": build_readiness_probe_image,
        "version-post-start-hook-init": build_version_post_start_hook_image,
        "operator-ubi": build_operator_ubi_image,
        "e2e": build_e2e_image,
    }[image_name]

    image_build_function(config)
    return 0


if __name__ == "__main__":
    sys.exit(main())
