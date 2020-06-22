import os
import json
import yaml
import sys
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
    return "\n{}\n{}\n{}\n".format(dashes,msg,dashes)


def dump_crd(crd_log: typing.TextIO):
    crdv1 = client.ApiextensionsV1beta1Api()
    try:
        headerS = header("CRD")
        crd_log.write(headers)
        mdb = crdv1.list_custom_resource_definition(pretty="true")
        body = yaml.dump(clean_nones(mdb.to_dict()))
        crd_log.write(body)
    except ApiException as e:
        print("Exception when calling ist_custom_resource_definition: %s\n" % e)
    return

def dump_persistent_volume(diagnosticFile: typing.TextIO):
    corev1 = client.CoreV1Api()
    try:
        headerS = header("Persistent Volume Claims")
        diagnosticFile.write(headerS)
        mdb = corev1.list_persistent_volume(pretty="true")
        body = yaml.dump(clean_nones(mdb.to_dict()))
        diagnosticFile.write(body)
    except ApiException as e:
        print("Exception when calling list_persistent_volume %s\n" % e)
    return

def dump_stateful_sets_namespaced(diagnosticFile: typing.TextIO, namespace: str):
    av1beta1 = client.AppsV1Api()
    try:
        headerS = header("Stateful Sets")
        diagnosticFile.write(headerS);
        mdb = av1beta1.list_namespaced_stateful_set(namespace, pretty="true")
        body = yaml.dump(clean_nones(mdb.to_dict()))
        diagnosticFile.write(body);
    except ApiException as e:
        print("Exception when calling list_namespaced_stateful_set: %s\n" % e)
    return 


def dump_pod_log_namespaced(namespace: str, name: str):
    corev1 = client.CoreV1Api()
    try:
        if name.startswith("mdb0"):

            podFile_mongoDBAgent = open("logs/{}-mongodb-agent.log".format(name), "w")
            podFile_mongod = open("logs/{}-mongod.log".format(name), "w")
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
            podFile = open("logs/{}.log".format(name), "w")
            log = corev1.read_namespaced_pod_log(
                name=name, namespace=namespace, pretty="true"
            )
            podFile.write(log)
            podFile.close()
    except ApiException as e:
        print("Exception when calling read_namespaced_pod_log: %s\n" % e)
    return 


def dump_pods_and_logs_namespaced(diagnosticFile: typing.TextIO, namespace: str):
    corev1 = client.CoreV1Api()
    try:
        headerS = header("Pods")
        diagnosticFile.write(headerS)
        pods = corev1.list_namespaced_pod(namespace)
        for pod in pods.items:
            name = pod.metadata.name
            headerS = header("Pod {}".format(name));
            body = yaml.dump(clean_nones(pod.to_dict()));
            diagnosticFile.write(headerS)
            diagnosticFile.write(body)
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
