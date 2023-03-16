package secret

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type builder struct {
	data            map[string][]byte
	dataType        corev1.SecretType
	labels          map[string]string
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
	b.data[key] = []byte(value)
	return b
}

func (b *builder) SetOwnerReferences(ownerReferences []metav1.OwnerReference) *builder {
	b.ownerReferences = ownerReferences
	return b
}

func (b *builder) SetLabels(labels map[string]string) *builder {
	newLabels := make(map[string]string, len(labels))
	for k, v := range labels {
		newLabels[k] = v
	}
	b.labels = newLabels
	return b
}

func (b *builder) SetByteData(stringData map[string][]byte) *builder {
	newStringDataBytes := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		newStringDataBytes[k] = v
	}
	b.data = newStringDataBytes
	return b
}
func (b *builder) SetStringMapToData(stringData map[string]string) *builder {
	newStringDataBytes := make(map[string][]byte, len(stringData))
	for k, v := range stringData {
		newStringDataBytes[k] = []byte(v)
	}
	b.data = newStringDataBytes
	return b
}

func (b *builder) SetDataType(dataType corev1.SecretType) *builder {
	b.dataType = dataType
	return b
}

func (b builder) Build() corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.name,
			Namespace:       b.namespace,
			OwnerReferences: b.ownerReferences,
			Labels:          b.labels,
		},
		Data: b.data,
		Type: b.dataType,
	}
}

func Builder() *builder {
	return &builder{
		labels:          map[string]string{},
		data:            map[string][]byte{},
		ownerReferences: []metav1.OwnerReference{},
	}
}
