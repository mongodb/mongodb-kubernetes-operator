#!/usr/bin/env python

from ghat.template import template_github_action
import sys
import ruamel.yaml

yaml = ruamel.yaml.YAML()

template_mapping = {
    ".action_templates/e2e-fork-template.yaml": ".github/workflows/e2e-fork.yml",
    ".action_templates/e2e-pr-template.yaml": ".github/workflows/e2e.yml",
    ".action_templates/e2e-single-template.yaml": ".github/workflows/e2e-dispatch.yml",
}


def main() -> int:
    for template in template_mapping:
        github_action = template_github_action(template)
        with open(template_mapping[template], "w+") as f:
            yaml.dump(github_action, f)
    return 0


if __name__ == "__main__":
    sys.exit(main())
