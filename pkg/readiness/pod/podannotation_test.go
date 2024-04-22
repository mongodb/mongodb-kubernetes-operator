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

// Currently seems like the appending functionality on the library used by the fake
// implementation to simulate JSONPatch is broken: https://github.com/evanphx/json-patch/issues/138
// The short term workaround is to have the annotation empty.

// TestPatchPodAnnotation verifies that patching of the pod works correctly
func TestPatchPodAnnotation(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-replica-set-0",
			Namespace: "test-ns",
			Annotations: map[string]string{
				mongodbAgentVersionAnnotation: "",
			},
		},
	})

	pod, _ := clientset.CoreV1().Pods("test-ns").Get(ctx, "my-replica-set-0", metav1.GetOptions{})
	assert.Empty(t, pod.Annotations[mongodbAgentVersionAnnotation])

	// adding the annotations
	assert.NoError(t, PatchPodAnnotation(ctx, "test-ns", 1, "my-replica-set-0", clientset))
	pod, _ = clientset.CoreV1().Pods("test-ns").Get(ctx, "my-replica-set-0", metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "1"}, pod.Annotations)

	// changing the annotations - no new annotations were added
	assert.NoError(t, PatchPodAnnotation(ctx, "test-ns", 2, "my-replica-set-0", clientset))
	pod, _ = clientset.CoreV1().Pods("test-ns").Get(ctx, "my-replica-set-0", metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "2"}, pod.Annotations)
}

func TestUpdatePodAnnotationPodNotFound(t *testing.T) {
	ctx := context.Background()
	assert.True(t, apiErrors.IsNotFound(PatchPodAnnotation(ctx, "wrong-ns", 1, "my-replica-set-0", fake.NewSimpleClientset())))
}
