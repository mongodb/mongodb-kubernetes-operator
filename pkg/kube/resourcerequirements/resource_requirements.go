package resourcerequirements

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	resourceMemory = "memory"
	resourceCpu    = "cpu"
)

// New returns a new corev1.ResourceRequirements with the specified arguments, and an error
// which indicates if there was a problem parsing the input
func New(limitsCpu, limitsMemory, requestsCpu, requestsMemory string) (corev1.ResourceRequirements, error) {
	limits, err := buildResourceList(limitsCpu, limitsMemory)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}

	requests, err := buildResourceList(requestsCpu, requestsMemory)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}, nil
}

// Default returns the default resource requirements for a container
func Default() (corev1.ResourceRequirements, error) {
	return New("1.0", "500m", "0.5", "400m")
}

func buildResourceList(cpu, memory string) (corev1.ResourceList, error) {
	cpuQuantity, err := resource.ParseQuantity(cpu)
	if err != nil {
		return nil, err
	}
	memoryQuantity, err := resource.ParseQuantity(memory)
	if err != nil {
		return nil, err
	}
	return corev1.ResourceList{
		resourceCpu:    cpuQuantity,
		resourceMemory: memoryQuantity,
	}, nil
}
