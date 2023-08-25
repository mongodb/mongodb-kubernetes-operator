package testdata

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Currently seems like the appending functionality on the library used by the fake
// implementation to simulate JSONPatch is broken: https://github.com/evanphx/json-patch/issues/138
// The short term workaround is to have the annotation empty.

// These are just k8s objects used for testing. Note, that these are defined in a non "_test.go" file as they are reused
// by other modules
func TestSecret(namespace, name string, version int) *corev1.Secret {
	// We don't need to create a full automation config - just the json with version field is enough
	deployment := fmt.Sprintf("{\"version\": %d}", version)
	secret := &corev1.Secret{Data: map[string][]byte{"cluster-config.json": []byte(deployment)}}
	secret.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
	return secret
}
func TestPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"agent.mongodb.com/version": "",
			},
		},
	}
}
