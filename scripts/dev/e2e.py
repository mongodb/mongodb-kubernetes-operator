from build_and_deploy_operator import (
    ignore_if_doesnt_exist,
    ignore_if_already_exists,
    load_yaml_from_file,
)  # TODO: put these function somewhere else
from dockerutil import build_and_push_image
from typing import Dict, Optional
from dev_config import load_config
from kubernetes import client, config
import argparse
import time


def _load_testrunner_service_account() -> Optional[Dict]:
    return load_yaml_from_file("deploy/testrunner/service_account.yaml")


def _load_testrunner_role() -> Optional[Dict]:
    return load_yaml_from_file("deploy/testrunner/role.yaml")


def _load_testrunner_role_binding() -> Optional[Dict]:
    return load_yaml_from_file("deploy/testrunner/role_binding.yaml")


def _load_testrunner_cluster_role_binding() -> Optional[Dict]:
    return load_yaml_from_file("deploy/testrunner/cluster_role_binding.yaml")


def _prepare_testrunner_environment():
    rbacv1 = client.RbacAuthorizationV1Api()
    corev1 = client.CoreV1Api()
    dev_config = load_config()

    _delete_testrunner_pod()

    print("Creating Role")
    ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role(
            dev_config.namespace, _load_testrunner_role()
        )
    )

    print("Creating Role Binding")
    ignore_if_already_exists(
        lambda: rbacv1.create_namespaced_role_binding(
            dev_config.namespace, _load_testrunner_role_binding()
        )
    )

    print("Creating Cluster Role Binding")
    ignore_if_already_exists(
        lambda: rbacv1.create_cluster_role_binding(
            _load_testrunner_cluster_role_binding()
        )
    )

    print("Creating ServiceAccount")
    ignore_if_already_exists(
        lambda: corev1.create_namespaced_service_account(
            dev_config.namespace, _load_testrunner_service_account()
        )
    )


def build_and_push_testrunner(repo_url: str, tag: str, path: str):
    return build_and_push_image(repo_url, tag, path, "testrunner")


def build_and_push_e2e(repo_url: str, tag: str, path: str):
    return build_and_push_image(repo_url, tag, path, "e2e")


def _delete_testrunner_pod() -> None:
    dev_config = load_config()
    corev1 = client.CoreV1Api()
    ignore_if_doesnt_exist(
        lambda: corev1.delete_namespaced_pod("test-runner", dev_config.namespace)
    )


def create_test_runner_pod(test: str):
    dev_config = load_config()
    corev1 = client.CoreV1Api()
    pod_body = _get_testrunner_pod_body(test)
    return corev1.create_namespaced_pod(dev_config.namespace, body=pod_body)


def _get_testrunner_pod_body(test: str) -> Dict:
    dev_config = load_config()
    return {
        "kind": "Pod",
        "metadata": {"name": "test-runner", "namespace": dev_config.namespace,},
        "spec": {
            "restartPolicy": "Never",
            "serviceAccountName": "test-runner",
            "containers": [
                {
                    "name": "test-runner",
                    "image": f"{dev_config.repo_url}/test-runner",
                    "imagePullPolicy": "Always",
                    "command": [
                        "./runner",
                        "--operatorImage",
                        f"{dev_config.repo_url}/mongodb-kubernetes-operator",
                        "--testImage",
                        f"{dev_config.repo_url}/e2e",
                        f"--test={test}",
                        f"--namespace={dev_config.namespace}",
                    ],
                }
            ],
        },
    }


def wait_for_pod_to_exist(corev1, name, namespace):
    print("Waiting for pod to be running")
    for i in range(10):
        try:
            pod = corev1.read_namespaced_pod(name, namespace)
            if pod.status.phase == "Running":
                return True
        except ApiException as e:
            pass
        time.sleep(5)
    raise Exception("Pod never got into Running state!")


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("test", help="Name of the test to run")
    return parser.parse_args()


if __name__ == "__main__":
    args = parse_args()
    config.load_kube_config()
    dev_config = load_config()
    build_and_push_testrunner(
        dev_config.repo_url, f"{dev_config.repo_url}/test-runner", "."
    )
    build_and_push_e2e(dev_config.repo_url, f"{dev_config.repo_url}/e2e", ".")

    _prepare_testrunner_environment()

    pod = create_test_runner_pod(args.test)
    corev1 = client.CoreV1Api()
    wait_for_pod_to_exist(corev1, "test-runner", dev_config.namespace)

    print(f"Running test: {args.test}")
    for line in corev1.read_namespaced_pod_log(
        "test-runner", dev_config.namespace, follow=True, _preload_content=False
    ).stream():
        print(line.decode("utf-8").rstrip())
