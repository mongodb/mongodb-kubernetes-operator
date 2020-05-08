import io
import os
import time
from typing import Dict, Optional

import yaml
from kubernetes import client, config
from kubernetes.client.rest import ApiException

from dev_config import DevConfig, load_config
from dockerfile_generator import render
from dockerutil import build_and_push_image


def _load_operator_service_account() -> Optional[Dict]:
    return load_yaml_from_file("deploy/service_account.yaml")


def _load_operator_role() -> Optional[Dict]:
    return load_yaml_from_file("deploy/role.yaml")


def _load_operator_role_binding() -> Optional[Dict]:
    return load_yaml_from_file("deploy/role_binding.yaml")


def _load_operator_deployment() -> Optional[Dict]:
    return load_yaml_from_file("deploy/operator.yaml")


def _load_mongodb_crd() -> Optional[Dict]:
    return load_yaml_from_file("deploy/crds/mongodb.com_mongodb_crd.yaml")


def load_yaml_from_file(path: str) -> Optional[Dict]:
    with open(path, "r") as f:
        return yaml.full_load(f.read())
    return None


def _ensure_crds():
    """
    ensure_crds makes sure that all the required CRDs have been created
    """
    crdv1 = client.ApiextensionsV1beta1Api()
    crd = _load_mongodb_crd()

    ignore_if_doesnt_exist(
        lambda: crdv1.delete_custom_resource_definition("mongodbs.mongodb.com")
    )

    # TODO: fix this, when calling create_custom_resource_definition, we get the error
    # ValueError("Invalid value for `conditions`, must not be `None`")
    # but the crd is still successfully created
    try:
        crdv1.create_custom_resource_definition(body=crd)
    except ValueError as e:
        pass

    print("Ensured CRDs")


def build_and_push_operator(repo_url: str, tag: str, path: str):
    """
    build_and_push_operator creates the Dockerfile for the operator
    and pushes it to the target repo
    """
    return build_and_push_image(repo_url, tag, path, "operator")


def _ignore_error_codes(fn, codes):
    try:
        fn()
    except ApiException as e:
        if e.status not in codes:
            raise


def ignore_if_already_exists(fn):
    """
    ignore_if_already_exists accepts a function and calls it,
    ignoring an Kubernetes API conflict errors
    """

    return _ignore_error_codes(fn, [409])


def ignore_if_doesnt_exist(fn):
    """
    ignore_if_doesnt_exist accepts a function and calls it,
    ignoring an Kubernetes API not found errors
    """
    return _ignore_error_codes(fn, [404])


def deploy_operator():
    """
    deploy_operator ensures the CRDs are created, and als creates all the required ServiceAccounts, Roles
    and RoleBindings for the operator, and then creates the operator deployment.
    """
    appsv1 = client.AppsV1Api()
    corev1 = client.CoreV1Api()
    rbacv1 = client.RbacAuthorizationV1Api()

    dev_config = load_config()
    _ensure_crds()

    ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role(
            dev_config.namespace, _load_operator_role()
        )
    )
    ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role_binding(
            dev_config.namespace, _load_operator_role_binding()
        )
    )
    ignore_if_already_exists(
        lambda: corev1.create_namespaced_service_account(
            dev_config.namespace, _load_operator_service_account()
        )
    )
    ignore_if_already_exists(
        lambda: appsv1.create_namespaced_deployment(
            dev_config.namespace, _load_operator_deployment()
        )
    )


def main():
    config.load_kube_config()
    dev_config = load_config()
    build_and_push_operator(
        dev_config.repo_url, f"{dev_config.repo_url}/mongodb-kubernetes-operator", "."
    )
    deploy_operator()


if __name__ == "__main__":
    main()
