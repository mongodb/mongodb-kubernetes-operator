from kubernetes.client.rest import ApiException
from kubernetes import client

from typing import Optional, List, Dict


def get_crds() -> Optional[Dict]:
    crdv1 = client.ApiextensionsV1beta1Api()
    try:
        crd = crdv1.list_custom_resource_definition(pretty="true")
    except ApiException as e:
        print("Exception when calling list_custom_resource_definition: %s\n" % e)
        return None
    return crd.to_dict()


def get_all_mongodb_namespaced(namespace: str) -> Optional[List]:
    customv1 = client.CustomObjectsApi()
    try:
        return list(
            customv1.list_namespaced_custom_object(
                "mongodbcommunity.mongodb.com",
                "v1",
                namespace,
                "mongodbcommunity",
                pretty=True,
            )["items"]
        )
    except ApiException as e:
        print("Exception when calling get_namespaced_custom_object %s\n" % e)
    return None


def get_persistent_volumes() -> Optional[Dict]:
    corev1 = client.CoreV1Api()
    try:
        pv = corev1.list_persistent_volume(pretty="true")
    except ApiException as e:
        print("Exception when calling list_persistent_volume %s\n" % e)
        return None
    return pv.to_dict()


def get_stateful_sets_namespaced(namespace: str) -> Optional[Dict]:
    av1beta1 = client.AppsV1Api()
    try:
        sst = av1beta1.list_namespaced_stateful_set(namespace, pretty="true")
    except ApiException as e:
        print("Exception when calling list_namespaced_stateful_set: %s\n" % e)
        return None
    return sst.to_dict()


def get_configmap_namespaced(namespace: str, name: str) -> Optional[Dict]:
    corev1 = client.CoreV1Api()
    try:
        config_map = corev1.read_namespaced_config_map(name, namespace, pretty="true")
    except ApiException as e:
        print("Exception when calling read_namespaced_config_map: %s\n" % e)
        return None
    return config_map.to_dict()


def get_secret_namespaced(namespace: str, name: str) -> Optional[Dict]:
    corev1 = client.CoreV1Api()
    try:
        secret = corev1.read_namespaced_secret(name, namespace, pretty="true")
    except ApiException as e:
        print("Exception when calling read_namespaced_secret: %s\n" % e)
        return None
    return secret.to_dict()


def get_pods_namespaced(namespace: str) -> Optional[List]:
    corev1 = client.CoreV1Api()
    try:
        pods = corev1.list_namespaced_pod(namespace)
    except ApiException as e:
        print("Exception when calling list_namespaced_pod: %s\n" % e)
        return None
    return pods.items


def get_pod_namespaced(namespace: str, pod_name: str) -> Optional[client.V1Pod]:
    corev1 = client.CoreV1Api()
    try:
        pod = corev1.read_namespaced_pod(name=pod_name, namespace=namespace)
    except ApiException as e:
        print("Exception when calling read_namespaced_pod: %s\n" % e)
        return None
    return pod


def get_pod_log_namespaced(
    namespace: str, pod_name: str, container_name: str
) -> Optional[str]:
    corev1 = client.CoreV1Api()
    try:
        log = corev1.read_namespaced_pod_log(
            name=pod_name, namespace=namespace, pretty="true", container=container_name
        )
    except ApiException as e:
        print("Exception when calling read_namespaced_pod_log: %s\n" % e)
        return None
    return log
