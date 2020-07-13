#!/usr/bin/env python

from kubernetes.client.rest import ApiException
from build_and_deploy_operator import (
    build_and_push_operator,
    deploy_operator,
    load_yaml_from_file,
)
import k8s_conditions
import k8s_request_data
import dump_diagnostic
from dockerutil import build_and_push_image
from typing import Dict
from dev_config import load_config, DevConfig
from kubernetes import client, config
import argparse
import time
import os
import sys
import yaml

TEST_RUNNER_NAME = "test-runner"


def _load_testrunner_service_account() -> Dict:
    return load_yaml_from_file("deploy/testrunner/service_account.yaml")


def _load_testrunner_role() -> Dict:
    return load_yaml_from_file("deploy/testrunner/role.yaml")


def _load_testrunner_role_binding() -> Dict:
    return load_yaml_from_file("deploy/testrunner/role_binding.yaml")


def _load_testrunner_cluster_role_binding() -> Dict:
    return load_yaml_from_file("deploy/testrunner/cluster_role_binding.yaml")


def _prepare_testrunner_environment(config_file: str) -> None:
    """
    _prepare_testrunner_environment ensures the ServiceAccount,
    Role and ClusterRole and bindings are created for the test runner.
    """
    rbacv1 = client.RbacAuthorizationV1Api()
    corev1 = client.CoreV1Api()
    dev_config = load_config(config_file)

    _delete_testrunner_pod(config_file)

    print("Creating Role")
    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role(
            dev_config.namespace, _load_testrunner_role()
        )
    )

    print("Creating Role Binding")
    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role_binding(
            dev_config.namespace, _load_testrunner_role_binding()
        )
    )

    print("Creating Cluster Role Binding")
    k8s_conditions.ignore_if_already_exists(
        lambda: rbacv1.create_cluster_role_binding(
            _load_testrunner_cluster_role_binding()
        )
    )

    print("Creating ServiceAccount")
    k8s_conditions.ignore_if_already_exists(
        lambda: corev1.create_namespaced_service_account(
            dev_config.namespace, _load_testrunner_service_account()
        )
    )


def create_kube_config() -> None:
    """Replicates the local kubeconfig file (pointed at by KUBECONFIG),
    as a ConfigMap."""
    corev1 = client.CoreV1Api()
    print("Creating kube-config ConfigMap")

    svc = corev1.read_namespaced_service("kubernetes", "default")

    kube_config_path = os.getenv("KUBECONFIG")
    if kube_config_path is None:
        raise ValueError("kube_config_path must not be None")

    with open(kube_config_path) as fd:
        kube_config = yaml.safe_load(fd.read())

    if kube_config is None:
        raise ValueError("kube_config_path must not be None")

    kube_config["clusters"][0]["cluster"]["server"] = "https://" + svc.spec.cluster_ip
    kube_config = yaml.safe_dump(kube_config)
    data = {"kubeconfig": kube_config}
    config_map = client.V1ConfigMap(
        metadata=client.V1ObjectMeta(name="kube-config"), data=data
    )

    k8s_conditions.ignore_if_already_exists(
        lambda: corev1.create_namespaced_config_map("default", config_map)
    )


def build_and_push_testrunner(repo_url: str, tag: str, path: str) -> None:
    """
    build_and_push_testrunner builds and pushes the test runner
    image.
    """
    build_and_push_image(repo_url, tag, path, "testrunner")


def build_and_push_e2e(repo_url: str, tag: str, path: str) -> None:
    """
    build_and_push_e2e builds and pushes the e2e image.
    """
    build_and_push_image(repo_url, tag, path, "e2e")


def build_and_push_prehook(repo_url: str, tag: str, path: str) -> None:
    """
    build_and_push_prehook builds and pushes the pre-stop-hook image.
    """
    build_and_push_image(repo_url, tag, path, "prehook")


def _delete_testrunner_pod(config_file: str) -> None:
    """
    _delete_testrunner_pod deletes the test runner pod
    if it already exists.
    """
    dev_config = load_config(config_file)
    corev1 = client.CoreV1Api()
    k8s_conditions.ignore_if_doesnt_exist(
        lambda: corev1.delete_namespaced_pod(TEST_RUNNER_NAME, dev_config.namespace)
    )


def create_test_runner_pod(
    test: str,
    config_file: str,
    tag: str,
    perform_cleanup: str,
    test_runner_image_name: str,
) -> None:
    """
    create_test_runner_pod creates the pod which will run all of the tests.
    """
    dev_config = load_config(config_file)
    corev1 = client.CoreV1Api()
    pod_body = _get_testrunner_pod_body(
        test, config_file, tag, perform_cleanup, test_runner_image_name
    )

    if not k8s_conditions.wait(
        lambda: corev1.list_namespaced_pod(
            dev_config.namespace, field_selector=f"metadata.name=={TEST_RUNNER_NAME}",
        ),
        lambda pod_list: len(pod_list.items) == 0,
        timeout=30,
        sleep_time=0.5,
    ):
        raise Exception(
            "Execution timed out while waiting for the existing pod to be deleted"
        )

    if not k8s_conditions.call_eventually_succeeds(
        lambda: corev1.create_namespaced_pod(dev_config.namespace, body=pod_body),
        sleep_time=10,
        timeout=60,
        exceptions_to_ignore=ApiException,
    ):
        raise Exception("Could not create test_runner pod!")


def wait_for_pod_to_be_running(
    corev1: client.CoreV1Api, name: str, namespace: str
) -> None:
    print("Waiting for pod to be running")
    if not k8s_conditions.wait(
        lambda: corev1.read_namespaced_pod(name, namespace),
        lambda pod: pod.status.phase == "Running",
        sleep_time=5,
        timeout=90,
        exceptions_to_ignore=ApiException,
    ):
        raise Exception("Pod never got into Running state!")


def _get_testrunner_pod_body(
    test: str,
    config_file: str,
    tag: str,
    perform_cleanup: str,
    test_runner_image_name: str,
) -> Dict:
    dev_config = load_config(config_file)
    return {
        "kind": "Pod",
        "metadata": {"name": TEST_RUNNER_NAME, "namespace": dev_config.namespace,},
        "spec": {
            "restartPolicy": "Never",
            "serviceAccountName": TEST_RUNNER_NAME,
            "containers": [
                {
                    "name": "test-runner",
                    "image": f"{dev_config.repo_url}/{test_runner_image_name}:{tag}",
                    "imagePullPolicy": "Always",
                    "command": [
                        "./runner",
                        "--operatorImage",
                        f"{dev_config.repo_url}/{dev_config.operator_image}:{tag}",
                        "--preHookImage",
                        f"{dev_config.repo_url}/{dev_config.prestop_hook_image}:{tag}",
                        "--testImage",
                        f"{dev_config.repo_url}/{dev_config.e2e_image}:{tag}",
                        f"--test={test}",
                        f"--namespace={dev_config.namespace}",
                        f"--performCleanup={perform_cleanup}",
                    ],
                }
            ],
        },
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--test", help="Name of the test to run")
    parser.add_argument(
        "--install-operator",
        help="Install the operator instead of assuming one already exists",
        action="store_true",
    )
    parser.add_argument(
        "--build-images",
        help="Build testrunner, e2e and prestop-hook images",
        action="store_true",
    )
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
    parser.add_argument("--config_file", help="Path to the config file")
    return parser.parse_args()


def build_and_push_images(args: argparse.Namespace, dev_config: DevConfig) -> None:
    test_runner_name = dev_config.testrunner_image
    if args.install_operator:
        build_and_push_operator(
            dev_config.repo_url,
            f"{dev_config.repo_url}/{dev_config.operator_image}:{args.tag}",
            ".",
        )
        deploy_operator()

    if args.build_images:
        build_and_push_testrunner(
            dev_config.repo_url,
            "{}/{}:{}".format(dev_config.repo_url, test_runner_name, args.tag),
            ".",
        )
        build_and_push_e2e(
            dev_config.repo_url,
            "{}/{}:{}".format(dev_config.repo_url, dev_config.e2e_image, args.tag),
            ".",
        )
        build_and_push_prehook(
            dev_config.repo_url,
            "{}/{}:{}".format(
                dev_config.repo_url, dev_config.prestop_hook_image, args.tag
            ),
            ".",
        )


def prepare_and_run_testrunner(args: argparse.Namespace, dev_config: DevConfig) -> None:
    test_runner_name = dev_config.testrunner_image
    _prepare_testrunner_environment(args.config_file)

    create_test_runner_pod(
        args.test, args.config_file, args.tag, args.perform_cleanup, test_runner_name,
    )
    corev1 = client.CoreV1Api()

    wait_for_pod_to_be_running(
        corev1, TEST_RUNNER_NAME, dev_config.namespace,
    )

    # stream all of the pod output as the pod is running
    for line in corev1.read_namespaced_pod_log(
        TEST_RUNNER_NAME, dev_config.namespace, follow=True, _preload_content=False
    ).stream():
        print(line.decode("utf-8").rstrip())


def main() -> int:
    args = parse_args()
    config.load_kube_config()

    dev_config = load_config(args.config_file)
    create_kube_config()

    try:
        build_and_push_images(args, dev_config)
        prepare_and_run_testrunner(args, dev_config)
    finally:
        if not args.skip_dump_diagnostic:
            dump_diagnostic.dump_all(dev_config.namespace)

    corev1 = client.CoreV1Api()
    if not k8s_conditions.wait(
        lambda: corev1.read_namespaced_pod(TEST_RUNNER_NAME, dev_config.namespace),
        lambda pod: pod.status.phase == "Succeeded",
        sleep_time=5,
        timeout=60,
        exceptions_to_ignore=ApiException,
    ):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
