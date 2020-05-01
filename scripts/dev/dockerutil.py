import docker
from dockerfile_generator import render
import os


def build_image(repo_url: str, tag: str, path):
    client = docker.from_env()
    print(f"Building image: {tag}")
    client.images.build(tag=tag, path=path)
    print("Successfully built image!")


def push_image(tag: str):
    client = docker.from_env()
    print(f"Pushing image: {tag}")
    for line in client.images.push(tag, stream=True):
        print(line.decode("utf-8").rstrip())


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
