package pod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

type mockPoller struct{}

func (m mockPoller) Poll(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	elapsedTime := time.Duration(0)
	for timeout >= elapsedTime {
		done, err := condition()
		if err != nil {
			return fmt.Errorf("error in condition func: %+v", err)
		}
		elapsedTime += interval
		if done {
			return nil
		}
	}
	return fmt.Errorf("timed out!")
}

func TestWaitForPhase(t *testing.T) {
	mockedClient := client.NewClient(client.NewMockedClient())
	testPod := newPod(corev1.PodRunning)
	err := mockedClient.Update(context.TODO(), &testPod)
	assert.NoError(t, err)
	_, err = waitForPhase(mockedClient, types.NamespacedName{Name: testPod.Name, Namespace: testPod.Namespace}, time.Second*5, time.Minute*5, corev1.PodRunning, mockPoller{})
	assert.NoError(t, err)

	testPod = newPod(corev1.PodFailed)
	err = mockedClient.Update(context.TODO(), &testPod)
	_, err = waitForPhase(mockedClient, types.NamespacedName{Name: testPod.Name, Namespace: testPod.Namespace}, time.Second*5, time.Minute*5, corev1.PodRunning, mockPoller{})
	assert.Error(t, err)
}

func newPod(phase corev1.PodPhase) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},

		Spec: corev1.PodSpec{},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}
}

type mockStreamer struct {
	logs string
}

func (m mockStreamer) Stream() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader(m.logs)), nil
}

func TestGetLogs(t *testing.T) {
	tests := []struct {
		expected string
	}{
		{expected: "Hello World"},
		{expected: "Line 1\nLine2\nLine3"},
		{expected: "Some other log line."},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		err := GetLogs(&b, mockStreamer{logs: tt.expected})
		assert.NoError(t, err)
		assert.Equal(t, tt.expected+"\n", b.String())
	}
}
