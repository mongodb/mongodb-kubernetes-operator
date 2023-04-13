package merge

import (
	corev1 "k8s.io/api/core/v1"
)

// ServiceSpec merges two ServiceSpecs together.
// The implementation merges Selector instead of overriding it.
func ServiceSpec(defaultSpec, overrideSpec corev1.ServiceSpec) corev1.ServiceSpec {
	mergedSpec := defaultSpec
	mergedSpec.Selector = StringToStringMap(defaultSpec.Selector, overrideSpec.Selector)

	if len(overrideSpec.Ports) != 0 {
		mergedSpec.Ports = overrideSpec.Ports
	}

	if len(overrideSpec.Type) != 0 {
		mergedSpec.Type = overrideSpec.Type
	}

	if len(overrideSpec.LoadBalancerIP) != 0 {
		mergedSpec.LoadBalancerIP = overrideSpec.LoadBalancerIP
	}

	if overrideSpec.LoadBalancerClass != nil {
		mergedSpec.LoadBalancerClass = overrideSpec.LoadBalancerClass
	}

	if len(overrideSpec.ExternalName) != 0 {
		mergedSpec.ExternalName = overrideSpec.ExternalName
	}

	if len(overrideSpec.ExternalTrafficPolicy) != 0 {
		mergedSpec.ExternalTrafficPolicy = overrideSpec.ExternalTrafficPolicy
	}

	if overrideSpec.InternalTrafficPolicy != nil {
		mergedSpec.InternalTrafficPolicy = overrideSpec.InternalTrafficPolicy
	}

	if overrideSpec.PublishNotReadyAddresses {
		mergedSpec.PublishNotReadyAddresses = overrideSpec.PublishNotReadyAddresses
	}

	if overrideSpec.HealthCheckNodePort != 0 {
		mergedSpec.HealthCheckNodePort = overrideSpec.HealthCheckNodePort
	}

	if len(overrideSpec.LoadBalancerSourceRanges) != 0 {
		mergedSpec.LoadBalancerSourceRanges = overrideSpec.LoadBalancerSourceRanges
	}

	if len(overrideSpec.ExternalIPs) != 0 {
		mergedSpec.ExternalIPs = overrideSpec.ExternalIPs
	}

	if overrideSpec.SessionAffinityConfig != nil {
		mergedSpec.SessionAffinityConfig = overrideSpec.SessionAffinityConfig
	}

	if len(overrideSpec.SessionAffinity) != 0 {
		mergedSpec.SessionAffinity = overrideSpec.SessionAffinity
	}

	if len(overrideSpec.ClusterIP) != 0 {
		mergedSpec.ClusterIP = overrideSpec.ClusterIP
	}

	if len(overrideSpec.ClusterIPs) != 0 {
		mergedSpec.ClusterIPs = overrideSpec.ClusterIPs
	}

	return mergedSpec
}
