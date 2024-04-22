package agent

import (
	"context"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
}

func TestAllReachedGoalState(t *testing.T) {
	ctx := context.Background()
	sts, err := statefulset.NewBuilder().SetName("sts").SetNamespace("test-ns").Build()
	assert.NoError(t, err)

	t.Run("Returns true if all pods are not found", func(t *testing.T) {
		ready, err := AllReachedGoalState(ctx, sts, mockPodGetter{}, 3, 3, zap.S())
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("Returns true if all pods are ready", func(t *testing.T) {
		ready, err := AllReachedGoalState(ctx, sts, mockPodGetter{pods: []corev1.Pod{
			createPodWithAgentAnnotation("3"),
			createPodWithAgentAnnotation("3"),
			createPodWithAgentAnnotation("3"),
		}}, 3, 3, zap.S())
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("Returns false if one pod is not ready", func(t *testing.T) {
		ready, err := AllReachedGoalState(ctx, sts, mockPodGetter{pods: []corev1.Pod{
			createPodWithAgentAnnotation("2"),
			createPodWithAgentAnnotation("3"),
			createPodWithAgentAnnotation("3"),
		}}, 3, 3, zap.S())
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("Returns true when the pods are not found", func(t *testing.T) {
		ready, err := AllReachedGoalState(ctx, sts, mockPodGetter{shouldReturnNotFoundError: true}, 3, 3, zap.S())
		assert.NoError(t, err)
		assert.True(t, ready)
	})
}

func TestReachedGoalState(t *testing.T) {
	t.Run("Pod reaches goal state when annotation is present", func(t *testing.T) {
		assert.True(t, ReachedGoalState(createPodWithAgentAnnotation("2"), 2, zap.S()))
		assert.True(t, ReachedGoalState(createPodWithAgentAnnotation("4"), 4, zap.S()))
		assert.True(t, ReachedGoalState(createPodWithAgentAnnotation("20"), 20, zap.S()))
	})

	t.Run("Pod does not reach goal state when there is a mismatch", func(t *testing.T) {
		assert.False(t, ReachedGoalState(createPodWithAgentAnnotation("2"), 4, zap.S()))
		assert.False(t, ReachedGoalState(createPodWithAgentAnnotation("3"), 7, zap.S()))
		assert.False(t, ReachedGoalState(createPodWithAgentAnnotation("10"), 1, zap.S()))
	})

	t.Run("Pod does not reach goal state when annotation is not present", func(t *testing.T) {
		assert.False(t, ReachedGoalState(corev1.Pod{}, 10, zap.S()))
	})
}

func createPodWithAgentAnnotation(versionStr string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				podAnnotationAgentVersion: versionStr,
			},
		},
	}
}

type mockPodGetter struct {
	pods                      []corev1.Pod
	currPodIndex              int
	shouldReturnNotFoundError bool
}

func (m mockPodGetter) GetPod(context.Context, client.ObjectKey) (corev1.Pod, error) {
	if m.shouldReturnNotFoundError || m.currPodIndex >= len(m.pods) {
		return corev1.Pod{}, notFoundError()
	}

	pod := m.pods[m.currPodIndex]

	return pod, nil
}

func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}
