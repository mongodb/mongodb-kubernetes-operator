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


def dump_crd(crd_log: typing.TextIO) -> str:
    crdv1 = client.ApiextensionsV1beta1Api()
    retVal=""
    try:
        headerS = header("CRD")
        retVal += headerS
        crd_log.write(retVal)
        mdb = crdv1.list_custom_resource_definition(pretty="true")
        body = yaml.dump(clean_nones(mdb.to_dict()))
        retVal += body
        crd_log.write(body)
    except ApiException as e:
        print("Exception when calling ist_custom_resource_definition: %s\n" % e)
    return retVal

def dump_persistent_volume(diagnosticFile: typing.TextIO) -> str:
    corev1 = client.CoreV1Api()
    retVal= ""
    try:
        headerS = header("Persistent Volume Claims")
        retVal += headerS
        diagnosticFile.write(headerS)
        mdb = corev1.list_persistent_volume(pretty="true")
        boidy = yaml.dump(clean_nones(mdb.to_dict()))
        retVal += body
        diagnosticFile.write(body)
    except ApiException as e:
        print("Exception when calling list_persistent_volume %s\n" % e)
    return retVal

def dump_stateful_sets_namespaced(diagnosticFile: typing.TextIO, namespace: str):
    av1beta1 = client.AppsV1Api()
    retVal = ""
    try:
        headerS = header("Stateful Sets")
        retVal += headerS
        diagnosticFile.write(headerS);
        mdb = av1beta1.list_namespaced_stateful_set(namespace, pretty="true")
        body = yaml.dump(clean_nones(mdb.to_dict()))
        retVal += body
        diagnosticFile.write(body);
    except ApiException as e:
        print("Exception when calling list_namespaced_stateful_set: %s\n" % e)
    return retVal


def dump_pod_log_namespaced(namespace: str, name: str):
    corev1 = client.CoreV1Api()
    retVal = ""
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
            retVal += log_mondodb_agent
            retVal += log_mongod
            podFile_mongoDBAgent.write(log_mongodb_agent)
            podFile_mongod.write(log_mongod)
            podFile_mongod.close()
            podFile_mongoDBAgent.close()
        elif name.startswith("mongodb-kubernetes-operator"):
            podFile = open("logs/{}.log".format(name), "w")
            log = corev1.read_namespaced_pod_log(
                name=name, namespace=namespace, pretty="true"
            )
            retVal += log
            podFile.write(log)
            podFile.close()
    except ApiException as e:
        print("Exception when calling read_namespaced_pod_log: %s\n" % e)
    return retVal


def dump_pods_and_logs_namespaced(diagnosticFile: typing.TextIO, namespace: str) -> str:
    corev1 = client.CoreV1Api()
    retVal= ""
    try:
        headerS = header("Pods")
        retVal += headerS
        diagnosticFile.write(headerS)
        pods = corev1.list_namespaced_pod(namespace)
        for pod in pods.items:
            name = pod.metadata.name
            headerS = header("Pod {}".format(name));
            retVal += headerS
            body = yaml.dump(clean_nones(pod.to_dict()));
            retVal += body
            diagnosticFile.write(headerS)
            diagnosticFile.write(body)
            dump_pod_log_namespaced(namespace, name)
    except ApiException as e:
        print("Exception when calling list_namespaced_pod: %s\n" % e)
    return retVal


def dump_all(namespace: str, to_file: bool):

    if not os.path.exists("logs"):
        os.makedirs("logs")

    if to_file:
        diagnosticFile = open("logs/diagnostics.txt", "w")
        crd_log = open("logs/crd.log", "w")
    else:
        diagnosticFile = sys.stderr
        crd_log = sys.stderr

    retVal = dump_crd(crd_log)
    retVal += dump_persistent_volume(diagnosticFile)
    retVal += dump_stateful_sets_namespaced(diagnosticFile, namespace)
    retVal += dump_pods_and_logs_namespaced(diagnosticFile, namespace)

    diagnosticFile.close()
    crd_log.close()
    return retVal
