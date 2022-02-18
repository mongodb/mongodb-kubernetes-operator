package service

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type builder struct {
	name                  string
	namespace             string
	clusterIp             string
	serviceType           corev1.ServiceType
	servicePort           []corev1.ServicePort
	labels                map[string]string
	loadBalancerIP        string
	publishNotReady       bool
	ownerReferences       []metav1.OwnerReference
	selector              map[string]string
	annotations           map[string]string
	externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType
}

func (b *builder) SetExternalTrafficPolicy(externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType) *builder {
	b.externalTrafficPolicy = externalTrafficPolicy
	return b
}

func (b *builder) SetLabels(labels map[string]string) *builder {
	b.labels = labels
	return b
}

func (b *builder) SetAnnotations(annotations map[string]string) *builder {
	b.annotations = annotations
	return b
}

func (b *builder) SetSelector(selector map[string]string) *builder {
	b.selector = selector
	return b
}

func (b *builder) SetName(name string) *builder {
	b.name = name
	return b
}

func (b *builder) SetNamespace(namespace string) *builder {
	b.namespace = namespace
	return b
}

func (b *builder) SetClusterIP(clusterIP string) *builder {
	b.clusterIp = clusterIP
	return b
}

func (b *builder) AddPort(port *corev1.ServicePort) *builder {
	if port != nil {
		b.servicePort = append(b.servicePort, *port)
	}

	return b
}

func (b *builder) SetServiceType(serviceType corev1.ServiceType) *builder {
	b.serviceType = serviceType
	return b
}

func (b *builder) SetLoadBalancerIP(ip string) *builder {
	b.loadBalancerIP = ip
	return b
}

func (b *builder) SetPublishNotReadyAddresses(publishNotReady bool) *builder {
	b.publishNotReady = publishNotReady
	return b
}

func (b *builder) SetOwnerReferences(ownerReferences []metav1.OwnerReference) *builder {
	b.ownerReferences = ownerReferences
	return b
}

func (b *builder) Build() corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.name,
			Namespace:       b.namespace,
			Labels:          b.labels,
			OwnerReferences: b.ownerReferences,
			Annotations:     b.annotations,
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: b.publishNotReady,
			ExternalTrafficPolicy:    b.externalTrafficPolicy,
			LoadBalancerIP:           b.loadBalancerIP,
			Type:                     b.serviceType,
			ClusterIP:                b.clusterIp,
			Ports:                    b.servicePort,
			Selector:                 b.selector,
		},
	}
}

func Builder() *builder {
	return &builder{
		labels:          map[string]string{},
		ownerReferences: []metav1.OwnerReference{},
		selector:        map[string]string{},
		annotations:     map[string]string{},
		servicePort:     []corev1.ServicePort{},
	}
}
