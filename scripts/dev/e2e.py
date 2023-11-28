#!/usr/bin/env python3

from kubernetes.client.rest import ApiException

import k8s_conditions
from typing import Dict
from dev_config import load_config, DevConfig, Distro
from kubernetes import client, config
import argparse
import sys
import yaml

TEST_POD_NAME = "e2e-test"
TEST_CLUSTER_ROLE_NAME = "e2e-test"
TEST_CLUSTER_ROLE_BINDING_NAME = "e2e-test"
TEST_SERVICE_ACCOUNT_NAME = "e2e-test"


def load_yaml_from_file(path: str) -> Dict:
    with open(path, "r") as f:
        return yaml.full_load(f.read())


def _load_test_service_account() -> Dict:
    return load_yaml_from_file("deploy/e2e/service_account.yaml")


def _load_test_role() -> Dict:
    return load_yaml_from_file("deploy/e2e/role.yaml")


def _load_test_role_binding() -> Dict:
    return load_yaml_from_file("deploy/e2e/role_binding.yaml")


def _prepare_test_environment(config_file: str) -> None:
    """
    _prepare_test_environment ensures that the old test pod is deleted
    and that namespace, cluster role, cluster role binding and service account
    are created for the test pod.
    """
    rbacv1 = client.RbacAuthorizationV1Api()
    corev1 = client.CoreV1Api()
    dev_config = load_config(config_file)

    _delete_test_pod(config_file)

    print("Creating Namespace")
    k8s_conditions.ignore_if_already_exists(
        lambda: corev1.create_namespace(
            client.V1Namespace(metadata=dict(name=dev_config.namespace))
        )
    )

    print("Creating Cluster Role")
    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_cluster_role(_load_test_role())
    )

    print("Creating Cluster Role Binding")
    role_binding = _load_test_role_binding()
    # set namespace specified in config.json
    role_binding["subjects"][0]["namespace"] = dev_config.namespace

    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_cluster_role_binding(role_binding)
    )

    print("Creating Service Account")
    service_account = _load_test_service_account()
    # set namespace specified in config.json
    service_account["metadata"]["namespace"] = dev_config.namespace

    k8s_conditions.ignore_if_already_exists(
        lambda: corev1.create_namespaced_service_account(
            dev_config.namespace, service_account
        )
    )


def create_test_pod(args: argparse.Namespace, dev_config: DevConfig) -> None:
    corev1 = client.CoreV1Api()
    test_pod = {
        "kind": "Pod",
        "metadata": {
            "name": TEST_POD_NAME,
            "namespace": dev_config.namespace,
            "labels": {"e2e-test": "true"},
        },
        "spec": {
            "restartPolicy": "Never",
            "serviceAccountName": "e2e-test",
            "containers": [
                {
                    "name": TEST_POD_NAME,
                    "image": f"{dev_config.repo_url}/{dev_config.e2e_image}:{args.tag}",
                    "imagePullPolicy": "Always",
                    "env": [
                        {
                            "name": "CLUSTER_WIDE",
                            "value": f"{args.cluster_wide}",
                        },
                        {
                            "name": "OPERATOR_IMAGE",
                            "value": f"{dev_config.repo_url}/{dev_config.operator_image_dev}:{args.tag}",
                        },
                        {
                            "name": "AGENT_IMAGE",
                            "value": f"{dev_config.repo_url}/{dev_config.agent_image_dev}:{args.tag}",
                        },
                        {
                            "name": "TEST_NAMESPACE",
                            "value": dev_config.namespace,
                        },
                        {
                            "name": "VERSION_UPGRADE_HOOK_IMAGE",
                            "value": f"{dev_config.repo_url}/{dev_config.version_upgrade_hook_image_dev}:{args.tag}",
                        },
                        {
                            "name": "READINESS_PROBE_IMAGE",
                            "value": f"{dev_config.repo_url}/{dev_config.readiness_probe_image_dev}:{args.tag}",
                        },
                        {
                            "name": "MONGODB_IMAGE",
                            "value": f"{dev_config.mongodb_image_name}",
                        },
                        {
                            "name": "MONGODB_REPO_URL",
                            "value": f"{dev_config.mongodb_image_repo_url}",
                        },
                        {
                            "name": "PERFORM_CLEANUP",
                            "value": f"{args.perform_cleanup}",
                        },
                    ],
                    "command": [
                        "go",
                        "test",
                        "-v",
                        "-timeout=45m",
                        "-failfast",
                        f"./test/e2e/{args.test}",
                    ],
                }
            ],
        },
    }
    if not k8s_conditions.wait(
        lambda: corev1.list_namespaced_pod(
            dev_config.namespace,
            field_selector=f"metadata.name=={TEST_POD_NAME}",
        ),
        lambda pod_list: len(pod_list.items) == 0,
        timeout=30,
        sleep_time=0.5,
    ):
        raise Exception(
            "Execution timed out while waiting for the existing pod to be deleted"
        )

    if not k8s_conditions.call_eventually_succeeds(
        lambda: corev1.create_namespaced_pod(dev_config.namespace, body=test_pod),
        sleep_time=10,
        timeout=60,
        exceptions_to_ignore=ApiException,
    ):
        raise Exception("Could not create test pod!")


def wait_for_pod_to_be_running(
    corev1: client.CoreV1Api, name: str, namespace: str
) -> None:
    print("Waiting for pod to be running")
    if not k8s_conditions.wait(
        lambda: corev1.read_namespaced_pod(name, namespace),
        lambda pod: pod.status.phase == "Running",
        sleep_time=5,
        timeout=240,
        exceptions_to_ignore=ApiException,
    ):

        pod = corev1.read_namespaced_pod(name, namespace)
        raise Exception("Pod never got into Running state: {}".format(pod))
    print("Pod is running")


def _delete_test_environment(config_file: str) -> None:
    """
    _delete_test_environment ensures that the cluster role, cluster role binding and service account
    for the test pod are deleted.
    """
    rbacv1 = client.RbacAuthorizationV1Api()
    corev1 = client.CoreV1Api()
    dev_config = load_config(config_file)

    k8s_conditions.ignore_if_doesnt_exist(
        lambda: rbacv1.delete_cluster_role(TEST_CLUSTER_ROLE_NAME)
    )

    k8s_conditions.ignore_if_doesnt_exist(
        lambda: rbacv1.delete_cluster_role_binding(TEST_CLUSTER_ROLE_BINDING_NAME)
    )

    k8s_conditions.ignore_if_doesnt_exist(
        lambda: corev1.delete_namespaced_service_account(
            TEST_SERVICE_ACCOUNT_NAME, dev_config.namespace
        )
    )


def _delete_test_pod(config_file: str) -> None:
    """
    _delete_test_pod deletes the test pod.
    """
    dev_config = load_config(config_file)
    corev1 = client.CoreV1Api()
    k8s_conditions.ignore_if_doesnt_exist(
        lambda: corev1.delete_namespaced_pod(TEST_POD_NAME, dev_config.namespace)
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--test", help="Name of the test to run")
    parser.add_argument(
        "--tag",
        help="Tag for the images, it will be the same for all images",
        type=str,
        default="latest",
    )
    parser.add_argument(
        "--skip-dump-diagnostic",
        help="Skip the dump of diagnostic information into files",
        action="store_true",
    )
    parser.add_argument(
        "--perform-cleanup",
        help="Cleanup the context after executing the tests",
        action="store_true",
    )
    parser.add_argument(
        "--cluster-wide",
        help="Watch all namespaces",
        type=lambda x: x.lower() == "true",
    )
    parser.add_argument(
        "--distro",
        help="The distro of images that should be used",
        type=str,
        default="ubi",
    )
    parser.add_argument("--config_file", help="Path to the config file")
    return parser.parse_args()


def prepare_and_run_test(args: argparse.Namespace, dev_config: DevConfig) -> None:
    _prepare_test_environment(args.config_file)
    create_test_pod(args, dev_config)
    corev1 = client.CoreV1Api()

    wait_for_pod_to_be_running(
        corev1,
        TEST_POD_NAME,
        dev_config.namespace,
    )

    # stream all of the pod output as the pod is running
    for line in corev1.read_namespaced_pod_log(
        TEST_POD_NAME, dev_config.namespace, follow=True, _preload_content=False
    ).stream():
        print(line.decode("utf-8").rstrip())


def main() -> int:
    args = parse_args()
    config.load_kube_config()

    dev_config = load_config(args.config_file, Distro.from_string(args.distro))
    prepare_and_run_test(args, dev_config)

    corev1 = client.CoreV1Api()
    if not k8s_conditions.wait(
        lambda: corev1.read_namespaced_pod(TEST_POD_NAME, dev_config.namespace),
        lambda pod: pod.status.phase == "Succeeded",
        sleep_time=5,
        timeout=60,
        exceptions_to_ignore=ApiException,
    ):
        return 1
    _delete_test_environment(args.config_file)
    return 0


if __name__ == "__main__":
    sys.exit(main())
