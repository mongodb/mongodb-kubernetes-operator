package configmap

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type builder struct {
	data            map[string]string
	name            string
	namespace       string
	ownerReferences []metav1.OwnerReference
}

func (b *builder) SetName(name string) *builder {
	b.name = name
	return b
}

func (b *builder) SetNamespace(namespace string) *builder {
	b.namespace = namespace
	return b
}

func (b *builder) SetField(key, value string) *builder {
	b.data[key] = value
	return b
}

func (b *builder) SetOwnerReferences(ownerReferences []metav1.OwnerReference) *builder {
	b.ownerReferences = ownerReferences
	return b
}

func (b builder) Build() corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.name,
			Namespace:       b.namespace,
			OwnerReferences: b.ownerReferences,
		},
		Data: b.data,
	}
}

func Builder() *builder {
	return &builder{
		data:            map[string]string{},
		ownerReferences: []metav1.OwnerReference{},
	}
}
