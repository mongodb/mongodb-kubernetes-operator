package sts

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type builder struct {
	name            string
	serviceName     string
	namespace       string
	labels          map[string]string
	ownerReference  []metav1.OwnerReference
	replicas        *int32
	matchLabels     map[string]string
	podTemplateSpec corev1.PodTemplateSpec
	volumeMounts    []corev1.VolumeMount
	volumeClaims    []corev1.PersistentVolumeClaim
}

func (s builder) SetLabels(labels map[string]string) builder {
	s.labels = labels
	return s
}

func (s builder) SetName(name string) builder {
	s.name = name
	return s
}

func (s builder) SetNamespace(namespace string) builder {
	s.namespace = namespace
	return s
}

func (s builder) SetOwnerReference(ownerReference []metav1.OwnerReference) builder {
	s.ownerReference = ownerReference
	return s
}

func (s builder) SetServiceName(serviceName string) builder {
	s.serviceName = serviceName
	return s
}

func (s builder) SetReplicas(replicas *int32) builder {
	s.replicas = replicas
	return s
}

func (s builder) SetMatchLabels(matchLabels map[string]string) builder {
	s.matchLabels = matchLabels
	return s
}

func (s builder) SetPodTemplateSpec(podTemplateSpec corev1.PodTemplateSpec) builder {
	s.podTemplateSpec = podTemplateSpec
	return s
}

func (s builder) Build() appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            s.name,
			Namespace:       s.namespace,
			Labels:          s.labels,
			OwnerReferences: s.ownerReference,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: s.serviceName,
			Replicas:    s.replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.matchLabels,
			},
			Template: s.podTemplateSpec,
		},
	}
}

func Builder() builder {
	return builder{
		labels:         map[string]string{},
		ownerReference: []metav1.OwnerReference{},
		volumeClaims:   []corev1.PersistentVolumeClaim{},
		volumeMounts:   []corev1.VolumeMount{},
	}
}
