import docker
from dockerfile_generator import render
import os
import json


def build_image(repo_url: str, tag: str, path):
    """
    build_image builds the image with the given tag
    """
    client = docker.from_env()
    print(f"Building image: {tag}")
    client.images.build(tag=tag, path=path)
    print("Successfully built image!")


def push_image(tag: str):
    """
    push_image pushes the given tag. It uses
    the current docker environment
    """
    client = docker.from_env()
    print(f"Pushing image: {tag}")
    progress = ""
    for line in client.images.push(tag, stream=True):
        print(push_image_formatted(line), end="")


def push_image_formatted(line) -> str:
    try:
        line = json.loads(line.strip())
    except ValueError:
        return ""

    to_skip = ("Preparing", "Waiting", "Layer already exists")
    if "status" in line:
        if line["status"] in to_skip:
            return ""
        if line["status"] == "Pushing":
            current = int(line["progressDetail"]["current"])
            total = int(line["progressDetail"]["total"])
            return "Complete: {:.2f}\n".format(current / total)

    return ""


def build_and_push_image(repo_url: str, tag: str, path: str, image_type: str):
    """
    build_and_push_operator creates the Dockerfile for the operator
    and pushes it to the target repo
    """
    dockerfile_text = render(image_type)
    with open(f"{path}/Dockerfile", "w") as f:
        f.write(dockerfile_text)

    build_image(repo_url, tag, path)
    os.remove(f"{path}/Dockerfile")
    push_image(tag)
