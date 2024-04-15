package secret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type reader struct {
	clientset kubernetes.Interface
}

func newKubernetesSecretReader(clientSet kubernetes.Interface) *reader {
	return &reader{clientset: clientSet}
}

func (r *reader) ReadSecret(ctx context.Context, namespace, secretName string) (*corev1.Secret, error) {
	return r.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
}
