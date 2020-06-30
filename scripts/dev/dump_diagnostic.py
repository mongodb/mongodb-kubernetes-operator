import os
import shutil
import yaml
import typing

import k8s_request_data


def clean_nones(value):
    """
    Recursively remove all None values from dictionaries and lists, and returns
    the result as a new dictionary or list.
    """
    if isinstance(value, list):
        return [clean_nones(x) for x in value if x is not None]
    if isinstance(value, dict):
        return {key: clean_nones(val) for key, val in value.items() if val is not None}
    return value


def header(msg: str) -> str:
    dashes = (
        "----------------------------------------------------------------------------"
    )
    return f"\n{dashes}\n{msg}\n{dashes}\n"


def dump_crd(crd_log: typing.TextIO):
    crd = k8s_request_data.get_crds()
    crd_log.write(header("CRD"))
    crd_log.write(yaml.dump(clean_nones(crd)))


def dump_persistent_volume(diagnostic_file: typing.TextIO):
    diagnostic_file.write(header("Persistent Volumes"))
    pv = k8s_request_data.get_persistent_volumes()
    diagnostic_file.write(yaml.dump(clean_nones(pv)))


def dump_stateful_sets_namespaced(diagnostic_file: typing.TextIO, namespace: str):
    diagnostic_file.write(header("Stateful Sets"))
    sst = k8s_request_data.get_stateful_sets_namespaced(namespace)
    diagnostic_file.write(yaml.dump(clean_nones(sst)))


def dump_pod_log_namespaced(namespace: str, name: str, containers: list):
    for container in containers:
        with open(f"logs/e2e/{name}-{container.name}.log", "w") as log_file:
            log_file.write(
                k8s_request_data.get_pod_log_namespaced(namespace, name, container.name)
            )


def dump_pods_and_logs_namespaced(diagnostic_file: typing.TextIO, namespace: str):
    pods = k8s_request_data.get_pods_namespaced(namespace)
    for pod in pods:
        name = pod.metadata.name
        diagnostic_file.write(header(f"Pod {name}"))
        diagnostic_file.write(yaml.dump(clean_nones(pod.to_dict())))
        dump_pod_log_namespaced(namespace, name, pod.spec.containers)


def dump_all(namespace: str):

    if os.path.exists("logs"):
        shutil.rmtree("logs")

    os.makedirs("logs")

    if not os.path.exists("logs/e2e"):
        os.makedirs("logs/e2e")

    with open("logs/e2e/diagnostics.txt", "w") as diagnostic_file:
        dump_persistent_volume(diagnostic_file)
        dump_stateful_sets_namespaced(diagnostic_file, namespace)
        dump_pods_and_logs_namespaced(diagnostic_file, namespace)

    with open("logs/e2e/crd.log", "w") as crd_log:
        dump_crd(crd_log)
