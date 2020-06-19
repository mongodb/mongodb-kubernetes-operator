import os
import json
import yaml
import typing

from kubernetes.client.rest import ApiException
from kubernetes import client


def clean_nones(value):
    """
    Recursively remove all None values from dictionaries and lists, and returns
    the result as a new dictionary or list.
    """
    if isinstance(value, list):
        return [clean_nones(x) for x in value if x is not None]
    elif isinstance(value, dict):
        return {key: clean_nones(val) for key, val in value.items() if val is not None}
    else:
        return value


def header(msg: str) -> str:
    dashes = (
        "----------------------------------------------------------------------------"
    )
    return f"{dashes}\n{msg}{dashes}"


def dump_crd(crd_log: typing.TextIO):
    crdv1 = client.ApiextensionsV1beta1Api()
    try:
        crd_log.write(header("CRD"))
        mdb = crdv1.list_custom_resource_definition(pretty="true")
        crd_log.write(yaml.dump(clean_nones(mdb.to_dict())))
    except ApiException as e:
        print("Exception when calling ist_custom_resource_definition: %s\n" % e)


def dump_persistent_volume(diagnosticFile: typing.TextIO):
    corev1 = client.CoreV1Api()
    try:
        diagnosticFile.write(header("Persistent Volume Claims"))
        mdb = corev1.list_persistent_volume(pretty="true")
        diagnosticFile.write(yaml.dump(clean_nones(mdb.to_dict())))
    except ApiException as e:
        print("Exception when calling list_persistent_volume %s\n" % e)


def dump_stateful_sets_namespaced(diagnosticFile: typing.TextIO, namespace: str):
    av1beta1 = client.AppsV1Api()
    try:
        diagnosticFile.write(header("Stateful Sets"))
        mdb = av1beta1.list_namespaced_stateful_set(namespace, pretty="true")
        diagnosticFile.write(yaml.dump(clean_nones(mdb.to_dict())))
    except ApiException as e:
        print("Exception when calling list_namespaced_stateful_set: %s\n" % e)


def dump_pod_log_namespaced(namespace: str, name: str):
    corev1 = client.CoreV1Api()
    try:
        if name.startswith("mdb0"):

            podFile_mongoDBAgent = open(f"logs/{name}-mongodb-agent.log", "w")
            podFile_mongod = open(f"logs/{name}-mongod.log", "w")
            log_mongodb_agent = corev1.read_namespaced_pod_log(
                name=name, namespace=namespace, pretty="true", container="mongodb-agent"
            )
            log_mongod = corev1.read_namespaced_pod_log(
                name=name, namespace=namespace, pretty="true", container="mongod"
            )
            podFile_mongoDBAgent.write(log_mongodb_agent)
            podFile_mongod.write(log_mongod)
            podFile_mongod.close()
            podFile_mongoDBAgent.close()
        elif name.startswith("mongodb-kubernetes-operator"):
            podFile = open(f"logs/{name}.log", "w")
            log = corev1.read_namespaced_pod_log(
                name=name, namespace=namespace, pretty="true"
            )
            podFile.write(log)
            podFile.close()
    except ApiException as e:
        print("Exception when calling read_namespaced_pod_log: %s\n" % e)


def dump_pods_and_logs_namespaced(diagnosticFile: typing.TextIO, namespace: str):
    corev1 = client.CoreV1Api()
    try:
        diagnosticFile.write(header("Pods"))
        pods = corev1.list_namespaced_pod(namespace)
        for pod in pods.items:
            name = pod.metadata.name
            diagnosticFile.write(header(f"Pod {name}"))
            diagnosticFile.write(yaml.dump(clean_nones(pod.to_dict())))
            dump_pod_log_namespaced(namespace, name)
    except ApiException as e:
        print("Exception when calling list_namespaced_pod: %s\n" % e)
    return


def dump_all(namespace: str):

    if not os.path.exists("logs"):
        os.makedirs("logs")

    diagnosticFile = open("logs/diagnostics.txt", "w")
    crd_log = open("logs/crd.log", "w")

    dump_crd(crd_log)
    dump_persistent_volume(diagnosticFile)
    dump_stateful_sets_namespaced(diagnosticFile, namespace)
    dump_pods_and_logs_namespaced(diagnosticFile, namespace)

    diagnosticFile.close()
    crd_log.close()
    return
