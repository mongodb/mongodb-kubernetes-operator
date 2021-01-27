package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type Streamer interface {
	Stream(ctx context.Context) (io.ReadCloser, error)
}

// CoreV1FollowStreamer returns a Streamer that will stream the logs to
// the given pod
func CoreV1FollowStreamer(pod corev1.Pod, corev1Interface typedCorev1.CoreV1Interface) Streamer {
	return corev1Interface.
		Pods(pod.Namespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow: true,
		})
}

// GetLogs will follow the logs of the provided pod to the given io.Writer until the pod has
// been terminated or has completed.
func GetLogs(writer io.Writer, streamer Streamer) error {
	podLogs, err := streamer.Stream(context.TODO())

	if err != nil {
		return errors.Errorf("could not open stream: %s", err)
	}

	defer podLogs.Close()

	sc := bufio.NewScanner(podLogs)
	for sc.Scan() {
		if _, err := fmt.Fprintln(writer, sc.Text()); err != nil {
			return err
		}
	}

	if sc.Err() != nil {
		return errors.Errorf("error from scanner: %s", sc.Err())
	}

	return nil
}

type Poller interface {
	Poll(interval, timeout time.Duration, condition wait.ConditionFunc) error
}

type waitPoller struct{}

func (p waitPoller) Poll(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	return wait.Poll(interval, timeout, condition)
}

// WaitForPhase waits for a pdo with the given namespacedName to exist, checking every interval with and using
// the provided timeout. The pod itself is returned and any error that occurred.
func WaitForPhase(c client.Client, namespacedName types.NamespacedName, interval, timeout time.Duration, podPhase corev1.PodPhase) (corev1.Pod, error) {
	return waitForPhase(c, namespacedName, interval, timeout, podPhase, waitPoller{})
}

func waitForPhase(c client.Client, namespacedName types.NamespacedName, interval, timeout time.Duration, podPhase corev1.PodPhase, poller Poller) (corev1.Pod, error) {
	pod := corev1.Pod{}
	err := poller.Poll(interval, timeout, func() (done bool, err error) {
		if err := c.Get(context.TODO(), namespacedName, &pod); err != nil {
			return false, err
		}
		return pod.Status.Phase == podPhase, nil
	})
	return pod, err
}
