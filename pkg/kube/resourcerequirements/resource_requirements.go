package resourcerequirements

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	resourceMemory = "memory"
	resourceCpu    = "cpu"
)

// Defaults returns the default resource requirements for a container
func Defaults() corev1.ResourceRequirements {
	// we can safely ignore the error as we are passing all valid values
	req, _ := newDefaultRequirements()
	return req
}

func newDefaultRequirements() (corev1.ResourceRequirements, error) {
	return newRequirements("1.0", "500M", "0.5", "400M")
}

// newRequirements returns a new corev1.ResourceRequirements with the specified arguments, and an error
// which indicates if there was a problem parsing the input
func newRequirements(limitsCpu, limitsMemory, requestsCpu, requestsMemory string) (corev1.ResourceRequirements, error) {
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

// buildDefaultStorageRequirements returns a corev1.ResourceList definition for storage requirements.
// This is used by the StatefulSet PersistentVolumeClaimTemplate.
// TODO: Allow to change these values.
func BuildDefaultStorageRequirements() corev1.ResourceList {
	g10, _ := resource.ParseQuantity("10G")
	res := corev1.ResourceList{}
	res[corev1.ResourceStorage] = g10
	return res
}

func BuildStorageRequirements(amount string) corev1.ResourceList {
	g10, _ := resource.ParseQuantity(amount)
	res := corev1.ResourceList{}
	res[corev1.ResourceStorage] = g10
	return res
}
