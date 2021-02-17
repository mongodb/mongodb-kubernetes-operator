import io
import os
from typing import Dict

import sys

import yaml
from kubernetes import client, config

from dev_config import DevConfig, load_config
from dockerfile_generator import render
from dockerutil import build_and_push_image

import k8s_conditions


def _load_operator_service_account() -> Dict:
    return load_yaml_from_file("deploy/operator/service_account.yaml")


def _load_operator_role() -> Dict:
    return load_yaml_from_file("deploy/operator/role.yaml")


def _load_operator_role_binding() -> Dict:
    return load_yaml_from_file("deploy/operator/role_binding.yaml")


def _load_operator_deployment(operator_image: str) -> Dict:
    operator = load_yaml_from_file("deploy/operator/operator.yaml")
    operator["spec"]["template"]["spec"]["containers"][0]["image"] = operator_image
    return operator


def _load_mongodb_crd() -> Dict:
    return load_yaml_from_file(
        "config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml"
    )


def load_yaml_from_file(path: str) -> Dict:
    with open(path, "r") as f:
        return yaml.full_load(f.read())


def _ensure_crds() -> None:
    """
    ensure_crds makes sure that all the required CRDs have been created
    """
    crdv1 = client.ApiextensionsV1beta1Api()
    crd = _load_mongodb_crd()

    k8s_conditions.ignore_if_doesnt_exist(
        lambda: crdv1.delete_custom_resource_definition("mongodbcommunity.mongodb.com")
    )

    # Make sure that the CRD has being deleted before trying to create it again
    if not k8s_conditions.wait(
        lambda: crdv1.list_custom_resource_definition(
            field_selector="metadata.name==mongodbcommunity.mongodb.com"
        ),
        lambda crd_list: len(crd_list.items) == 0,
        timeout=5,
        sleep_time=0.5,
    ):
        raise Exception("Execution timed out while waiting for the CRD to be deleted")

    # TODO: fix this, when calling create_custom_resource_definition, we get the error
    # ValueError("Invalid value for `conditions`, must not be `None`")
    # but the crd is still successfully created
    try:
        crdv1.create_custom_resource_definition(body=crd)
    except ValueError as e:
        pass

    print("Ensured CRDs")


def build_and_push_operator(repo_url: str, tag: str, path: str) -> None:
    """
    build_and_push_operator creates the Dockerfile for the operator
    and pushes it to the target repo
    """
    build_and_push_image(repo_url, tag, path, "operator")


def deploy_operator() -> None:
    """
    deploy_operator ensures the CRDs are created, and als creates all the required ServiceAccounts, Roles
    and RoleBindings for the operator, and then creates the operator deployment.
    """
    appsv1 = client.AppsV1Api()
    corev1 = client.CoreV1Api()
    rbacv1 = client.RbacAuthorizationV1Api()

    dev_config = load_config()
    _ensure_crds()

    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role(
            dev_config.namespace, _load_operator_role()
        )
    )
    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role_binding(
            dev_config.namespace, _load_operator_role_binding()
        )
    )
    k8s_conditions.ignore_if_already_exists(
        lambda: corev1.create_namespaced_service_account(
            dev_config.namespace, _load_operator_service_account()
        )
    )
    k8s_conditions.ignore_if_already_exists(
        lambda: appsv1.create_namespaced_deployment(
            dev_config.namespace,
            _load_operator_deployment(
                f"{dev_config.repo_url}/mongodb-kubernetes-operator"
            ),
        )
    )


def main() -> int:
    config.load_kube_config()
    dev_config = load_config()
    build_and_push_operator(
        dev_config.repo_url,
        f"{dev_config.repo_url}/mongodb-kubernetes-operator",
        ".",
    )
    deploy_operator()
    return 0


if __name__ == "__main__":
    sys.exit(main())
