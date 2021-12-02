#!/usr/bin/env python3

"""
Creates a new local go.mod file with updates as fetched from
dependency updates patches from dependabot.

Requirements:

- GITHUB_ACCESS_TOKEN: env variable needs to be set
- GITHUB_REPO: name of the org/repo to fetch the dependency updates patches from.
               defaults to `mongodb/mongodb-kubernetes-operator`.

Usage:

python scripts/ci/merge_dependabot_prs.py

-----------

After calling the script, a new file called `go.mod.updated` will be created in the
repository root. You can copy it over `go.mod` and run your usual tests:

```
cp go.mod.updated go.mod
go mod tidy
make test
```
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Dict, List, TextIO

from github import Github, PullRequest
from semver import compare

g = Github(os.environ["GITHUB_ACCESS_TOKEN"])
repo = g.get_repo(os.environ.get("GITHUB_REPO", "mongodb/mongodb-kubernetes-operator"))


MODFILE = "go.mod"


@dataclass
class GoModule:
    name: str
    version: str
    indirect: bool = False

    @staticmethod
    def from_line(line: str) -> GoModule:
        line = line.strip()
        indirect = False
        if "// indirect" in line:
            indirect = True

        line = line.removesuffix(" // indirect")  # type: ignore

        # print("line is", line)
        name, version = line.split()
        # Skip first v from version
        version = version[1:]

        return GoModule(name, version, indirect)

    def __str__(self) -> str:
        indirect_str = " // indirect" if self.indirect else ""
        version = "v" + self.version
        return "\t{} {}{}".format(self.name, version, indirect_str)


def get_dependency_pr() -> List[PullRequest.PullRequest]:
    open_prs = repo.get_pulls(state="open")
    prs = []
    for pr in open_prs:
        for label in pr.labels:
            if label.name == "dependencies":
                prs.append(pr)

    return prs


def extract_module_updated_from_patch(patch: str) -> Dict[str, GoModule]:
    splitted_patch = patch.split("\n")
    modules = {}
    for line in splitted_patch:
        if line.startswith("+"):
            module = GoModule.from_line(line.removeprefix("+"))  # type: ignore
            modules[module.name] = module

    return modules


def get_updated_modules_in_pr(
    prs: List[PullRequest.PullRequest],
) -> Dict[str, GoModule]:
    modules: Dict[str, GoModule] = {}
    for pr in prs:
        for file in pr.get_files():
            if file.filename == "go.mod":
                modules_this_pr = extract_module_updated_from_patch(file.patch)
                for _, module in modules_this_pr.items():
                    # Unimplemented
                    if module.version.endswith("incompatible"):
                        continue

                    if (
                        module.name not in modules
                        or compare(module.version, modules[module.name].version) > 0
                    ):
                        modules[module.name] = module
                        continue

    return modules


def read_modules() -> Dict:
    modules = {}
    modfile = open(MODFILE).readlines()
    reading = False
    for line in modfile:
        if line.startswith("require"):
            reading = True
            continue

        if line.startswith(")") and reading:
            break

        if reading:
            module = GoModule.from_line(line)
            modules[module.name] = module

    return modules


def write_require_block(fd: TextIO, modules: Dict[str, str]) -> None:
    fd.write("require (\n")
    for _, mod in modules.items():
        fd.write(str(mod) + "\n")
    fd.write(")\n")


def write_modules(modules: Dict[str, str], filename: str) -> None:
    modfile = open(MODFILE).readlines()
    newmodfile = open(filename, "w")
    skip = False
    for line in modfile:
        if line.startswith("require ("):
            write_require_block(newmodfile, modules)
            skip = True
            continue

        if skip and line.startswith(")"):
            skip = False
            continue

        if skip:
            continue
        newmodfile.write(line)


if __name__ == "__main__":
    current_modules = read_modules()
    dependency_prs = get_dependency_pr()
    updated_modules = get_updated_modules_in_pr(dependency_prs)
    current_modules.update(updated_modules)
    write_modules(current_modules, "go.mod.updated")
