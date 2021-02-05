import docker
from dockerfile_generator import render
import os
import json
from tqdm import tqdm

from typing import Union, Any, Optional


def build_image(repo_url: str, tag: str, path: str) -> None:
    """
    build_image builds the image with the given tag
    """
    client = docker.from_env()
    print(f"Building image: {tag}")
    client.images.build(tag=tag, path=path)
    print("Successfully built image!")


def push_image(tag: str) -> None:
    """
    push_image pushes the given tag. It uses
    the current docker environment
    """
    client = docker.from_env()
    print(f"Pushing image: {tag}")
    with tqdm(total=100, ascii=False) as progress_bar:
        last_percent = 0.0
        for line in client.images.push(tag, stream=True):
            percent = get_completion_percentage(line)
            if percent:
                progress_bar.update(percent - last_percent)
                last_percent = percent


def retag_image(
    old_repo_url: str,
    new_repo_url: str,
    old_tag: str,
    new_tag: str,
    path: str,
    labels: Optional[dict] = None,
    username: Optional[str] = None,
    password: Optional[str] = None,
    registry: Optional[str] = None,
) -> None:
    with open(f"{path}/Dockerfile", "w") as f:
        f.write(f"FROM {old_repo_url}:{old_tag}")
    client = docker.from_env()
    if all(value is not None for value in [username, password, registry]):
        client.login(username=username, password=password, registry=registry)

    image, _ = client.images.build(path=f"{path}", labels=labels, tag=new_tag)
    image.tag(new_repo_url, new_tag)
    os.remove(f"{path}/Dockerfile")

    # We do not want to republish an image that has not changed, so we check if the new
    # pair repo:tag already exists.
    try:
        image = client.images.pull(new_repo_url, new_tag)
        return
    # We also need to catch APIError as if the image has been recently deleted (uncommon, but might happen?)
    # we will get this kind of error:
    # docker.errors.APIError: 500 Server Error: Internal Server Error
    # ("unknown: Tag <tag> was deleted or has expired. To pull, revive via time machine"
    except (docker.errors.ImageNotFound, docker.errors.APIError) as e:
        pass
    print(f"Pushing to {new_repo_url}:{new_tag}")
    client.images.push(new_repo_url, new_tag)


def get_completion_percentage(line: Any) -> float:
    try:
        line = json.loads(line.strip().decode("utf-8"))
    except ValueError:
        return 0

    to_skip = ("Preparing", "Waiting", "Layer already exists")
    if "status" in line:
        if line["status"] in to_skip:
            return 0
        if line["status"] == "Pushing":
            try:
                current = float(line["progressDetail"]["current"])
                total = float(line["progressDetail"]["total"])
            except KeyError:
                return 0
            result = (current / total) * 100
            if result > 100.0:
                return 100.0
            return result
    return 0


def build_and_push_image(repo_url: str, tag: str, path: str, image_type: str) -> None:
    """
    build_and_push_operator creates the Dockerfile for the operator
    and pushes it to the target repo
    """
    dockerfile_text = render(image_type, ["."])
    with open(f"{path}/Dockerfile", "w") as f:
        f.write(dockerfile_text)

    build_image(repo_url, tag, path)
    os.remove(f"{path}/Dockerfile")
    push_image(tag)
