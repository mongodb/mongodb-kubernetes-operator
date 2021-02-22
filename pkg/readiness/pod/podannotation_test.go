package pod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestPatchPodAnnotation verifies that patching of the pod works correctly
func TestPatchPodAnnotation(t *testing.T) {
	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replica-set-0",
			Namespace: "test-ns",
		},
	})

	pod, _ := clientset.CoreV1().Pods("test-ns").Get(context.TODO(), "my-replica-set-0", metav1.GetOptions{})
	assert.Empty(t, pod.Annotations)

	// adding the annotations
	assert.NoError(t, PatchPodAnnotation("test-ns", 1, "my-replica-set-0", clientset))
	pod, _ = clientset.CoreV1().Pods("test-ns").Get(context.TODO(), "my-replica-set-0", metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "1"}, pod.Annotations)

	// changing the annotations - no new annotations were added
	assert.NoError(t, PatchPodAnnotation("test-ns", 2, "my-replica-set-0", clientset))
	pod, _ = clientset.CoreV1().Pods("test-ns").Get(context.TODO(), "my-replica-set-0", metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "2"}, pod.Annotations)
}

func TestUpdatePodAnnotationPodNotFound(t *testing.T) {
	assert.True(t, apiErrors.IsNotFound(PatchPodAnnotation("wrong-ns", 1, "my-replica-set-0", fake.NewSimpleClientset())))
}
