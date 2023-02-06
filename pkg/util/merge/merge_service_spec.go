package merge

import (
	corev1 "k8s.io/api/core/v1"
)

// ServiceSpec merges two ServiceSpecs together.
// The implementation does not override:
// - labels/selectors
// - cluster IPs
func ServiceSpec(defaultSpec, overrideSpec corev1.ServiceSpec) corev1.ServiceSpec {
	mergedSpec := overrideSpec
	mergedSpec.Selector = StringToStringMap(defaultSpec.Selector, overrideSpec.Selector)
	mergedSpec.ClusterIPs = defaultSpec.ClusterIPs
	mergedSpec.ClusterIP = defaultSpec.ClusterIP
	return mergedSpec
}
