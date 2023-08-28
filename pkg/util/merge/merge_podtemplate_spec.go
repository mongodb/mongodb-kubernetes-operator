package merge

import (
	"sort"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
	corev1 "k8s.io/api/core/v1"
)

func PodTemplateSpecs(original, override corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	merged := original

	merged.Annotations = StringToStringMap(original.Annotations, override.Annotations)
	merged.Labels = StringToStringMap(original.Labels, override.Labels)
	merged.Spec.Volumes = Volumes(original.Spec.Volumes, override.Spec.Volumes)
	merged.Spec.Containers = Containers(original.Spec.Containers, override.Spec.Containers)
	merged.Spec.InitContainers = Containers(original.Spec.InitContainers, override.Spec.InitContainers)

	if override.Spec.EphemeralContainers != nil {
		merged.Spec.EphemeralContainers = EphemeralContainers(original.Spec.EphemeralContainers, override.Spec.EphemeralContainers)
	}

	if override.Spec.RestartPolicy != "" {
		merged.Spec.RestartPolicy = override.Spec.RestartPolicy
	}

	if override.Spec.TerminationGracePeriodSeconds != nil {
		merged.Spec.TerminationGracePeriodSeconds = override.Spec.TerminationGracePeriodSeconds
	}
	if override.Spec.ActiveDeadlineSeconds != nil {
		merged.Spec.ActiveDeadlineSeconds = override.Spec.ActiveDeadlineSeconds
	}

	if override.Spec.DNSPolicy != "" {
		merged.Spec.DNSPolicy = override.Spec.DNSPolicy
	}

	if override.Spec.NodeSelector != nil {
		merged.Spec.NodeSelector = StringToStringMap(original.Spec.NodeSelector, override.Spec.NodeSelector)
	}

	if override.Spec.ServiceAccountName != "" {
		merged.Spec.ServiceAccountName = override.Spec.ServiceAccountName
	}

	if override.Spec.DeprecatedServiceAccount != "" {
		merged.Spec.DeprecatedServiceAccount = override.Spec.DeprecatedServiceAccount
	}

	if override.Spec.AutomountServiceAccountToken != nil {
		merged.Spec.AutomountServiceAccountToken = override.Spec.AutomountServiceAccountToken
	}

	if override.Spec.NodeName != "" {
		merged.Spec.NodeName = override.Spec.NodeName
	}

	if override.Spec.HostNetwork {
		merged.Spec.HostNetwork = override.Spec.HostNetwork
	}

	if override.Spec.HostPID {
		merged.Spec.HostPID = override.Spec.HostPID
	}

	if override.Spec.ShareProcessNamespace != nil {
		merged.Spec.ShareProcessNamespace = override.Spec.ShareProcessNamespace
	}

	if override.Spec.SecurityContext != nil {
		merged.Spec.SecurityContext = override.Spec.SecurityContext
	}

	if override.Spec.ImagePullSecrets != nil {
		merged.Spec.ImagePullSecrets = override.Spec.ImagePullSecrets
	}

	if override.Spec.Hostname != "" {
		merged.Spec.Hostname = override.Spec.Hostname
	}

	if override.Spec.Subdomain != "" {
		merged.Spec.Subdomain = override.Spec.Subdomain
	}

	if override.Spec.Affinity != nil {
		merged.Spec.Affinity = Affinity(original.Spec.Affinity, override.Spec.Affinity)
	}

	if override.Spec.SchedulerName != "" {
		merged.Spec.SchedulerName = override.Spec.SchedulerName
	}

	if override.Spec.Tolerations != nil {
		merged.Spec.Tolerations = override.Spec.Tolerations
	}

	merged.Spec.HostAliases = HostAliases(original.Spec.HostAliases, override.Spec.HostAliases)

	if override.Spec.PriorityClassName != "" {
		merged.Spec.PriorityClassName = override.Spec.PriorityClassName
	}

	if override.Spec.Priority != nil {
		merged.Spec.Priority = override.Spec.Priority
	}

	if override.Spec.DNSConfig != nil {
		merged.Spec.DNSConfig = PodDNSConfig(original.Spec.DNSConfig, override.Spec.DNSConfig)
	}

	if override.Spec.ReadinessGates != nil {
		merged.Spec.ReadinessGates = override.Spec.ReadinessGates
	}

	if override.Spec.RuntimeClassName != nil {
		merged.Spec.RuntimeClassName = override.Spec.RuntimeClassName
	}

	if override.Spec.EnableServiceLinks != nil {
		merged.Spec.EnableServiceLinks = override.Spec.EnableServiceLinks
	}

	if override.Spec.PreemptionPolicy != nil {
		merged.Spec.PreemptionPolicy = override.Spec.PreemptionPolicy
	}

	if override.Spec.Overhead != nil {
		merged.Spec.Overhead = override.Spec.Overhead
	}

	if override.Spec.TopologySpreadConstraints != nil {
		merged.Spec.TopologySpreadConstraints = TopologySpreadConstraints(original.Spec.TopologySpreadConstraints, override.Spec.TopologySpreadConstraints)
	}

	return merged
}

func TopologySpreadConstraints(original, override []corev1.TopologySpreadConstraint) []corev1.TopologySpreadConstraint {
	originalMap := createTopologySpreadConstraintMap(original)
	overrideMap := createTopologySpreadConstraintMap(override)

	mergedMap := map[string]corev1.TopologySpreadConstraint{}

	for k, v := range originalMap {
		mergedMap[k] = v
	}
	for k, v := range overrideMap {
		if originalValue, ok := mergedMap[k]; ok {
			mergedMap[k] = TopologySpreadConstraint(originalValue, v)
		} else {
			mergedMap[k] = v
		}
	}
	var mergedElements []corev1.TopologySpreadConstraint
	for _, v := range mergedMap {
		mergedElements = append(mergedElements, v)
	}
	return mergedElements
}

func TopologySpreadConstraint(original, override corev1.TopologySpreadConstraint) corev1.TopologySpreadConstraint {
	merged := original
	if override.LabelSelector != nil {
		merged.LabelSelector = override.LabelSelector
	}
	if override.MaxSkew != 0 {
		merged.MaxSkew = override.MaxSkew
	}
	if override.WhenUnsatisfiable != "" {
		merged.WhenUnsatisfiable = override.WhenUnsatisfiable
	}
	return merged
}

func createTopologySpreadConstraintMap(constraints []corev1.TopologySpreadConstraint) map[string]corev1.TopologySpreadConstraint {
	m := make(map[string]corev1.TopologySpreadConstraint)
	for _, v := range constraints {
		m[v.TopologyKey] = v
	}
	return m
}

// HostAliases merges two slices of HostAliases together. Any shared hostnames with a given
// ip are merged together into fewer entries.
func HostAliases(originalAliases, overrideAliases []corev1.HostAlias) []corev1.HostAlias {
	m := make(map[string]corev1.HostAlias)
	for _, original := range originalAliases {
		m[original.IP] = original
	}

	for _, override := range overrideAliases {
		if _, ok := m[override.IP]; ok {
			var mergedHostNames []string
			mergedHostNames = append(mergedHostNames, m[override.IP].Hostnames...)
			for _, hn := range override.Hostnames {
				if !contains.String(mergedHostNames, hn) {
					mergedHostNames = append(mergedHostNames, hn)
				}
			}
			m[override.IP] = corev1.HostAlias{
				IP:        override.IP,
				Hostnames: mergedHostNames,
			}
		} else {
			m[override.IP] = override
		}
	}

	var mergedHostAliases []corev1.HostAlias
	for _, v := range m {
		mergedHostAliases = append(mergedHostAliases, v)
	}

	sort.SliceStable(mergedHostAliases, func(i, j int) bool {
		return mergedHostAliases[i].IP < mergedHostAliases[j].IP
	})

	return mergedHostAliases
}

func PodDNSConfig(originalDNSConfig, overrideDNSConfig *corev1.PodDNSConfig) *corev1.PodDNSConfig {
	if overrideDNSConfig == nil {
		return originalDNSConfig
	}

	if originalDNSConfig == nil {
		return overrideDNSConfig
	}

	merged := originalDNSConfig.DeepCopy()

	if overrideDNSConfig.Options != nil {
		merged.Options = overrideDNSConfig.Options
	}

	if overrideDNSConfig.Nameservers != nil {
		merged.Nameservers = StringSlices(merged.Nameservers, overrideDNSConfig.Nameservers)
	}

	if overrideDNSConfig.Searches != nil {
		merged.Searches = StringSlices(merged.Searches, overrideDNSConfig.Searches)
	}

	return merged
}
