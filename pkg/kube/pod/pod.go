package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TailLogs will follow the logs of the provided pod to the given io.Writer until the pod has
// been terminated or has completed.
func TailLogs(pod corev1.Pod, writer io.Writer, corev1Interface typedCorev1.CoreV1Interface) error {

	// CoreV1Interface is required as the newer k8sclient.Client has no mechanism for accessing
	// pod logs currently.
	podLogs, err := corev1Interface.
		Pods(pod.Namespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow: true,
		}).Stream()

	if err != nil {
		return fmt.Errorf("error in opening stream: %v", err)
	}

	defer podLogs.Close()

	sc := bufio.NewScanner(podLogs)
	for sc.Scan() {
		if _, err := fmt.Fprint(writer, sc.Text()); err != nil {
			return err
		}
	}

	if sc.Err() != nil {
		return fmt.Errorf("error from scanner: %+v", sc.Err())
	}

	return nil
}

// WaitForExistence waits for a pdo with the given namespacedName to exist, checking every interval with and using
// the provided timeout. The pod itself is returned and any error that occurred.
func WaitForExistence(c k8sClient.Client, namespacedName types.NamespacedName, interval, timeout time.Duration) (corev1.Pod, error) {
	pod := corev1.Pod{}
	err := wait.Poll(interval, timeout, func() (done bool, err error) {
		if err := c.Get(context.TODO(), namespacedName, &pod); err != nil {
			return false, err
		}
		return true, nil
	})
	return pod, err
}
