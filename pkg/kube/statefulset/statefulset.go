package statefulset

import (
	"sort"

	"github.com/imdario/mergo"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	notFound = -1
)

type Getter interface {
	GetStatefulSet(objectKey client.ObjectKey) (appsv1.StatefulSet, error)
}

type Updater interface {
	UpdateStatefulSet(sts appsv1.StatefulSet) error
}

type Creator interface {
	CreateStatefulSet(sts appsv1.StatefulSet) error
}

type Deleter interface {
	DeleteStatefulSet(objectKey client.ObjectKey) error
}

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
}

type GetUpdateCreateDeleter interface {
	Getter
	Updater
	Creator
	Deleter
}

// CreateOrUpdate creates the given StatefulSet if it doesn't exist,
// or updates it if it does.
func CreateOrUpdate(getUpdateCreator GetUpdateCreator, sts appsv1.StatefulSet) error {
	_, err := getUpdateCreator.GetStatefulSet(types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateStatefulSet(sts)
		}
		return err
	}
	return getUpdateCreator.UpdateStatefulSet(sts)
}

// GetAndUpdate applies the provided function to the most recent version of the object
func GetAndUpdate(getUpdater GetUpdater, nsName types.NamespacedName, updateFunc func(*appsv1.StatefulSet)) error {
	sts, err := getUpdater.GetStatefulSet(nsName)
	if err != nil {
		return err
	}
	// apply the function on the most recent version of the resource
	updateFunc(&sts)
	return getUpdater.UpdateStatefulSet(sts)
}

// VolumeMountData contains values required for the MountVolume function
type VolumeMountData struct {
	Name      string
	MountPath string
	Volume    corev1.Volume
}

func CreateVolumeFromConfigMap(name, sourceName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: sourceName,
				},
			},
		},
	}
}

func CreateVolumeFromSecret(name, sourceName string, options ...func(v *corev1.Volume)) corev1.Volume {
	volumeMount := &corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: sourceName,
			},
		},
	}
	for _, option := range options {
		option(volumeMount)
	}
	return *volumeMount

}

func CreateVolumeFromEmptyDir(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			// No options EmptyDir means default storage medium and size.
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// CreateVolumeMount returns a corev1.VolumeMount with options.
func CreateVolumeMount(name, path string, options ...func(*corev1.VolumeMount)) corev1.VolumeMount {
	volumeMount := &corev1.VolumeMount{
		Name:      name,
		MountPath: path,
	}
	for _, option := range options {
		option(volumeMount)
	}
	return *volumeMount
}

func WithSecretDefaultMode(mode *int32) func(*corev1.Volume) {
	return func(v *corev1.Volume) {
		if v.VolumeSource.Secret == nil {
			v.VolumeSource.Secret = &corev1.SecretVolumeSource{}
		}
		v.VolumeSource.Secret.DefaultMode = mode
	}
}

// WithSubPath sets the SubPath for this VolumeMount
func WithSubPath(subPath string) func(*corev1.VolumeMount) {
	return func(v *corev1.VolumeMount) {
		v.SubPath = subPath
	}
}

// WithReadOnly sets the ReadOnly attribute of this VolumeMount
func WithReadOnly(readonly bool) func(*corev1.VolumeMount) {
	return func(v *corev1.VolumeMount) {
		v.ReadOnly = readonly
	}
}

func IsReady(sts appsv1.StatefulSet, expectedReplicas int) bool {
	allUpdated := int32(expectedReplicas) == sts.Status.UpdatedReplicas
	allReady := int32(expectedReplicas) == sts.Status.ReadyReplicas
	return allUpdated && allReady
}

type Modification func(*appsv1.StatefulSet)

func New(mods ...Modification) appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	for _, mod := range mods {
		mod(&sts)
	}
	return sts
}

func Apply(funcs ...Modification) func(*appsv1.StatefulSet) {
	return func(sts *appsv1.StatefulSet) {
		for _, f := range funcs {
			f(sts)
		}
	}
}

func WithName(name string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Name = name
	}
}

func WithNamespace(namespace string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Namespace = namespace
	}
}

func WithServiceName(svcName string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.ServiceName = svcName
	}
}

func WithLabels(labels map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Labels = copyMap(labels)
	}
}
func WithMatchLabels(matchLabels map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		if set.Spec.Selector == nil {
			set.Spec.Selector = &metav1.LabelSelector{}
		}
		set.Spec.Selector.MatchLabels = copyMap(matchLabels)
	}
}
func WithOwnerReference(ownerRefs []metav1.OwnerReference) Modification {
	ownerReference := make([]metav1.OwnerReference, len(ownerRefs))
	copy(ownerReference, ownerRefs)
	return func(set *appsv1.StatefulSet) {
		set.OwnerReferences = ownerReference
	}
}

func WithReplicas(replicas int) Modification {
	stsReplicas := int32(replicas)
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.Replicas = &stsReplicas
	}
}

func WithUpdateStrategyType(strategyType appsv1.StatefulSetUpdateStrategyType) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: strategyType,
		}
	}
}

func WithPodSpecTemplate(templateFunc func(*corev1.PodTemplateSpec)) Modification {
	return func(set *appsv1.StatefulSet) {
		template := &set.Spec.Template
		templateFunc(template)
	}
}

func WithVolumeClaim(name string, f func(*corev1.PersistentVolumeClaim)) Modification {
	return func(set *appsv1.StatefulSet) {
		idx := findVolumeClaimIndexByName(name, set.Spec.VolumeClaimTemplates)
		if idx == notFound {
			set.Spec.VolumeClaimTemplates = append(set.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{})
			idx = len(set.Spec.VolumeClaimTemplates) - 1
		}
		pvc := &set.Spec.VolumeClaimTemplates[idx]
		f(pvc)
	}
}

func mergedValue(original, modified interface{}) interface{} {
	if modified != nil {
		return modified
	}
	return original
}

func mergedString(original, modified string) string {
	if modified != "" {
		return modified
	}
	return original
}

func mergedLabelSelector(original, modified *metav1.LabelSelector) *metav1.LabelSelector {
	if modified == nil {
		return original
	}
	for key, val := range modified.MatchLabels {
		original.MatchLabels[key] = val
	}
	// Selector.MatchExpressions
	for _, expression := range modified.MatchExpressions {
		if index := findMatchExpressionByKey(expression.Key, original.MatchExpressions); index == notFound {
			// Add
			original.MatchExpressions = append(original.MatchExpressions, expression)
		} else {
			original.MatchExpressions[index] = expression
		}
	}
	return original
}

func mergedContainer(original, modified corev1.Container) corev1.Container {
	// args
	if len(modified.Args) > 0 {
		original.Args = modified.Args
	}
	//command
	if len(modified.Command) > 0 {
		original.Command = modified.Command
	}

	// env
	for _, envVar := range modified.Env {
		if index := findEnvVarByName(envVar.Name, original.Env); index == notFound {
			original.Env = append(original.Env, envVar)
		} else {
			original.Env[index] = envVar
		}
	}

	// envFrom
	if len(modified.EnvFrom) > 0 {
		original.EnvFrom = modified.EnvFrom
	}

	// image and imagepullpolicy
	original.Image = mergedString(original.Image, modified.Image)
	if modified.ImagePullPolicy != "" {
		original.ImagePullPolicy = modified.ImagePullPolicy
	}

	// lifecycle
	if val, ok := mergedValue(original.Lifecycle, modified.Lifecycle).(*corev1.Lifecycle); ok {
		original.Lifecycle = val
	}

	//lifevenessprobe
	if val, ok := mergedValue(original.LivenessProbe, modified.LivenessProbe).(*corev1.Probe); ok {
		original.LivenessProbe = val
	}

	//ports
	for _, port := range modified.Ports {
		if index := findContainerPortByPort(port.ContainerPort, original.Ports); index == notFound {
			original.Ports = append(original.Ports, port)
		} else {
			original.Ports[index].HostIP = mergedString(original.Ports[index].HostIP, port.HostIP)
			original.Ports[index].Name = mergedString(original.Ports[index].Name, port.Name)
			if port.Protocol != "" {
				original.Ports[index].Protocol = port.Protocol
			}
			if port.HostPort != 0 {
				original.Ports[index].HostPort = port.HostPort
			}
		}
	}

	//ReadinessProbe
	if val, ok := mergedValue(original.ReadinessProbe, modified.ReadinessProbe).(*corev1.Probe); ok {
		original.ReadinessProbe = val
	}

	// resources - limits
	if len(modified.Resources.Limits) > 0 {
		original.Resources.Limits = modified.Resources.Limits
	}

	// resources - requests
	if len(modified.Resources.Requests) > 0 {
		original.Resources.Requests = modified.Resources.Requests
	}

	//SecurityContext
	if val, ok := mergedValue(original.SecurityContext, modified.SecurityContext).(*corev1.SecurityContext); ok {
		original.SecurityContext = val
	}

	//StartupProbe
	if val, ok := mergedValue(original.StartupProbe, modified.StartupProbe).(*corev1.Probe); ok {
		original.StartupProbe = val
	}

	// stdin
	original.Stdin = modified.Stdin
	//stdinOnce
	original.StdinOnce = modified.StdinOnce

	// terminationMessagePath
	original.TerminationMessagePath = mergedString(original.TerminationMessagePath, modified.TerminationMessagePath)

	// terminationMEssagePolicy
	if modified.TerminationMessagePolicy != "" {
		original.TerminationMessagePolicy = modified.TerminationMessagePolicy
	}

	//TTY
	original.TTY = modified.TTY

	//volumedevices
	for _, device := range modified.VolumeDevices {
		if index := findVolumeDeviceByName(device.Name, original.VolumeDevices); index == notFound {
			original.VolumeDevices = append(original.VolumeDevices, device)
		} else {
			original.VolumeDevices[index].DevicePath = mergedString(original.VolumeDevices[index].DevicePath, device.DevicePath)
		}
	}

	//volumemounts
	for _, mount := range modified.VolumeMounts {
		if index := findVolumeMountsByMountPath(mount.MountPath, original.VolumeMounts); index == notFound {
			original.VolumeMounts = append(original.VolumeMounts, mount)
		} else {
			original.VolumeMounts[index].Name = mergedString(original.VolumeMounts[index].Name, mount.Name)
			original.VolumeMounts[index].SubPath = mergedString(original.VolumeMounts[index].SubPath, mount.SubPath)
			original.VolumeMounts[index].SubPathExpr = mergedString(original.VolumeMounts[index].SubPathExpr, mount.SubPathExpr)
			original.VolumeMounts[index].ReadOnly = mount.ReadOnly
			if val, ok := mergedValue(original.VolumeMounts[index].MountPropagation, mount.MountPropagation).(*corev1.MountPropagationMode); ok {
				original.VolumeMounts[index].MountPropagation = val
			}
		}
	}

	//workingdir
	original.WorkingDir = mergedString(original.WorkingDir, modified.WorkingDir)

	return original
}

func createVolumeClaimMap(volumeMounts []corev1.PersistentVolumeClaim) map[string]corev1.PersistentVolumeClaim {
	mountMap := make(map[string]corev1.PersistentVolumeClaim)
	for _, m := range volumeMounts {
		mountMap[m.Name] = m
	}
	return mountMap
}

func mergeVolumeClaimTemplates(original, modified []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	defaultMountsMap := createVolumeClaimMap(original)
	customMountsMap := createVolumeClaimMap(modified)
	var mergedVolumes []corev1.PersistentVolumeClaim
	for _, defaultMount := range defaultMountsMap {
		if customMount, ok := customMountsMap[defaultMount.Name]; ok {
			// needs merge
			if err := mergo.Merge(&defaultMount, customMount, mergo.WithAppendSlice, mergo.WithOverride); err != nil {
				return nil
			}
		}
		mergedVolumes = append(mergedVolumes, defaultMount)
	}
	for _, customMount := range customMountsMap {
		if _, ok := defaultMountsMap[customMount.Name]; ok {
			// already merged
			continue
		}
		mergedVolumes = append(mergedVolumes, customMount)
	}
	sort.SliceStable(mergedVolumes, func(i, j int) bool {
		return mergedVolumes[i].Name < mergedVolumes[j].Name
	})
	return mergedVolumes
}

// Merges the change provided by the user on top of what we already built
func WithStatefulSetSpec(spec appsv1.StatefulSetSpec) Modification {
	return func(sts *appsv1.StatefulSet) {
		// Replicas
		if spec.Replicas != nil {
			sts.Spec.Replicas = spec.Replicas
		}
		// Selector
		sts.Spec.Selector = mergedLabelSelector(sts.Spec.Selector, spec.Selector)

		// ServiceName
		if spec.ServiceName != "" {
			sts.Spec.ServiceName = spec.ServiceName
		}
		// RevisionHistoryLimit
		if spec.RevisionHistoryLimit != nil {
			sts.Spec.RevisionHistoryLimit = spec.RevisionHistoryLimit
		}
		// UpdateStrategy
		if spec.UpdateStrategy.Type != "" {
			sts.Spec.UpdateStrategy = spec.UpdateStrategy
		}
		if spec.UpdateStrategy.RollingUpdate != nil {
			sts.Spec.UpdateStrategy.RollingUpdate = spec.UpdateStrategy.RollingUpdate
		}

		// PodManagementPolicy
		if spec.PodManagementPolicy != "" {
			sts.Spec.PodManagementPolicy = spec.PodManagementPolicy
		}

		sts.Spec.VolumeClaimTemplates = mergeVolumeClaimTemplates(sts.Spec.VolumeClaimTemplates, spec.VolumeClaimTemplates)

		//VolumeClaimTemplates
		if len(spec.VolumeClaimTemplates) != 0 {
			sts.Spec.VolumeClaimTemplates = spec.VolumeClaimTemplates
		}

		//PodTemplateSpec
		// - ObjectMeta
		// TODO
		// - PodSpec
		podSpec := spec.Template.Spec
		// -- actieveDeadLineSeconds
		if podSpec.ActiveDeadlineSeconds != nil {
			sts.Spec.Template.Spec.ActiveDeadlineSeconds = podSpec.ActiveDeadlineSeconds
		}
		// -- Affinity
		if val, ok := mergedValue(sts.Spec.Template.Spec.Affinity, podSpec.Affinity).(*corev1.Affinity); ok {
			sts.Spec.Template.Spec.Affinity = val
		}
		// -- automountService
		if val, ok := mergedValue(sts.Spec.Template.Spec.AutomountServiceAccountToken, podSpec.AutomountServiceAccountToken).(*bool); ok {
			sts.Spec.Template.Spec.AutomountServiceAccountToken = val
		}
		// -- Containers
		for _, container := range podSpec.Containers {
			if index := findContainerByName(container.Name, sts.Spec.Template.Spec.Containers); index == notFound {
				sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, container)
			} else {
				sts.Spec.Template.Spec.Containers[index] = mergedContainer(sts.Spec.Template.Spec.Containers[index], container)
			}
		}

		//DNSConfig
		if val, ok := mergedValue(sts.Spec.Template.Spec.DNSConfig, podSpec.DNSConfig).(*corev1.PodDNSConfig); ok {
			sts.Spec.Template.Spec.DNSConfig = val
		}
		// EnableServiceLinks
		if val, ok := mergedValue(sts.Spec.Template.Spec.EnableServiceLinks, podSpec.EnableServiceLinks).(*bool); ok {
			sts.Spec.Template.Spec.EnableServiceLinks = val
		}
		//EphemeralContainers
		for _, container := range podSpec.EphemeralContainers {
			if index := findEphemeralContainerByName(container.Name, sts.Spec.Template.Spec.EphemeralContainers); index == notFound {
				sts.Spec.Template.Spec.EphemeralContainers = append(sts.Spec.Template.Spec.EphemeralContainers, container)
			} else {
				sts.Spec.Template.Spec.EphemeralContainers[index].EphemeralContainerCommon = corev1.EphemeralContainerCommon(mergedContainer(corev1.Container(sts.Spec.Template.Spec.EphemeralContainers[index].EphemeralContainerCommon), corev1.Container(container.EphemeralContainerCommon)))
				sts.Spec.Template.Spec.EphemeralContainers[index].TargetContainerName = mergedString(sts.Spec.Template.Spec.EphemeralContainers[index].TargetContainerName, container.TargetContainerName)
			}
		}

		// hostAliases
		for _, hostalias := range podSpec.HostAliases {
			if index := findHostAliasByIp(hostalias.IP, sts.Spec.Template.Spec.HostAliases); index == notFound {
				sts.Spec.Template.Spec.HostAliases = append(sts.Spec.Template.Spec.HostAliases, hostalias)
			} else {
				sts.Spec.Template.Spec.HostAliases[index] = hostalias
			}

		}

		// host
		sts.Spec.Template.Spec.HostIPC = podSpec.HostIPC
		sts.Spec.Template.Spec.HostNetwork = podSpec.HostNetwork
		sts.Spec.Template.Spec.HostPID = podSpec.HostPID

		sts.Spec.Template.Spec.Hostname = mergedString(sts.Spec.Template.Spec.Hostname, podSpec.Hostname)

		// ImagePullSecrets
		for _, ips := range podSpec.ImagePullSecrets {
			if index := findImagePullSecretByName(ips.Name, sts.Spec.Template.Spec.ImagePullSecrets); index == notFound {
				sts.Spec.Template.Spec.ImagePullSecrets = append(sts.Spec.Template.Spec.ImagePullSecrets, ips)
			} else {
				sts.Spec.Template.Spec.ImagePullSecrets[index] = ips
			}

		}

		// InitContainers
		for _, container := range podSpec.InitContainers {
			if index := findContainerByName(container.Name, sts.Spec.Template.Spec.InitContainers); index == notFound {
				sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, container)
			} else {
				sts.Spec.Template.Spec.InitContainers[index] = mergedContainer(sts.Spec.Template.Spec.InitContainers[index], container)
			}
		}

		//nodename
		sts.Spec.Template.Spec.NodeName = mergedString(sts.Spec.Template.Spec.NodeName, podSpec.NodeName)

		//nodeselector
		if len(podSpec.NodeSelector) != 0 {
			sts.Spec.Template.Spec.NodeSelector = podSpec.NodeSelector
		}
		//overhead
		if len(podSpec.Overhead) != 0 {
			sts.Spec.Template.Spec.Overhead = podSpec.Overhead
		}
		// preemption policy
		if val, ok := mergedValue(sts.Spec.Template.Spec.PreemptionPolicy, podSpec.PreemptionPolicy).(*corev1.PreemptionPolicy); ok {
			sts.Spec.Template.Spec.PreemptionPolicy = val
		}
		//priority
		if val, ok := mergedValue(sts.Spec.Template.Spec.Priority, podSpec.Priority).(*int32); ok {
			sts.Spec.Template.Spec.Priority = val
		}
		// PriorityClassName
		sts.Spec.Template.Spec.PriorityClassName = mergedString(sts.Spec.Template.Spec.PriorityClassName, podSpec.PriorityClassName)

		// Readiness gates
		if len(podSpec.ReadinessGates) != 0 {
			sts.Spec.Template.Spec.ReadinessGates = podSpec.ReadinessGates
		}

		//restartpolicy
		if podSpec.RestartPolicy != "" {
			sts.Spec.Template.Spec.RestartPolicy = podSpec.RestartPolicy
		}
		// RunTimeClassName
		if val, ok := mergedValue(sts.Spec.Template.Spec.RuntimeClassName, podSpec.RuntimeClassName).(*string); ok {
			sts.Spec.Template.Spec.RuntimeClassName = val
		}

		//SchedulerName
		sts.Spec.Template.Spec.SchedulerName = mergedString(sts.Spec.Template.Spec.SchedulerName, podSpec.SchedulerName)

		//Security Context
		if podSpec.SecurityContext != nil {
			sts.Spec.Template.Spec.SecurityContext = podSpec.SecurityContext
		}

		// ServiceAccountName
		sts.Spec.Template.Spec.ServiceAccountName = mergedString(sts.Spec.Template.Spec.ServiceAccountName, podSpec.ServiceAccountName)

		//shareProcessNamespace
		if val, ok := mergedValue(sts.Spec.Template.Spec.ShareProcessNamespace, podSpec.ShareProcessNamespace).(*bool); ok {
			sts.Spec.Template.Spec.ShareProcessNamespace = val
		}

		//subdomain
		sts.Spec.Template.Spec.Subdomain = mergedString(sts.Spec.Template.Spec.Subdomain, podSpec.Subdomain)

		if podSpec.TerminationGracePeriodSeconds != nil {
			sts.Spec.Template.Spec.TerminationGracePeriodSeconds = podSpec.TerminationGracePeriodSeconds
		}

		// tolerations
		if len(podSpec.Tolerations) > 0 {
			sts.Spec.Template.Spec.Tolerations = podSpec.Tolerations
		}

		// TopologySpreadContraints
		for _, constraint := range podSpec.TopologySpreadConstraints {
			if index := findTopologySpreadConstraintByKey(constraint.TopologyKey, sts.Spec.Template.Spec.TopologySpreadConstraints); index == notFound {
				sts.Spec.Template.Spec.TopologySpreadConstraints = append(sts.Spec.Template.Spec.TopologySpreadConstraints, constraint)
			} else {
				// merge
				sts.Spec.Template.Spec.TopologySpreadConstraints[index].MaxSkew = constraint.MaxSkew
				sts.Spec.Template.Spec.TopologySpreadConstraints[index].WhenUnsatisfiable = constraint.WhenUnsatisfiable
				sts.Spec.Template.Spec.TopologySpreadConstraints[index].LabelSelector = mergedLabelSelector(sts.Spec.Template.Spec.TopologySpreadConstraints[index].LabelSelector, constraint.LabelSelector)

			}

		}

		// volumes
		for _, volume := range podSpec.Volumes {
			if index := findVolumeByName(volume.Name, sts.Spec.Template.Spec.Volumes); index == notFound {
				sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, volume)
			} else {
				sts.Spec.Template.Spec.Volumes[index] = volume
			}

		}
	}
}

func findMatchExpressionByKey(key string, matchExpressions []metav1.LabelSelectorRequirement) int {
	for idx, expression := range matchExpressions {
		if expression.Key == key {
			return idx
		}
	}
	return notFound
}

func findVolumeClaimIndexByName(name string, pvcs []corev1.PersistentVolumeClaim) int {
	for idx, pvc := range pvcs {
		if pvc.Name == name {
			return idx
		}
	}
	return notFound
}

func findHostAliasByIp(ip string, hostAliases []corev1.HostAlias) int {
	for idx, alias := range hostAliases {
		if alias.IP == ip {
			return idx
		}
	}
	return notFound
}

func findImagePullSecretByName(name string, ips []corev1.LocalObjectReference) int {
	for idx, localObjectReference := range ips {
		if localObjectReference.Name == name {
			return idx
		}
	}
	return notFound
}

func findTopologySpreadConstraintByKey(key string, tsc []corev1.TopologySpreadConstraint) int {
	for idx, constraint := range tsc {
		if constraint.TopologyKey == key {
			return idx
		}
	}
	return notFound
}

func findVolumeByName(name string, volumes []corev1.Volume) int {
	for idx, volume := range volumes {
		if volume.Name == name {
			return idx
		}
	}
	return notFound
}

func findContainerByName(name string, containers []corev1.Container) int {
	for idx, container := range containers {
		if container.Name == name {
			return idx
		}
	}
	return notFound
}

func findEphemeralContainerByName(name string, containers []corev1.EphemeralContainer) int {
	for idx, container := range containers {
		if container.Name == name {
			return idx
		}
	}
	return notFound
}

func findEnvVarByName(name string, vars []corev1.EnvVar) int {
	for idx, val := range vars {
		if val.Name == name {
			return idx
		}
	}
	return notFound
}

func findContainerPortByPort(port int32, ports []corev1.ContainerPort) int {
	for idx, containerPort := range ports {
		if containerPort.ContainerPort == port {
			return idx
		}
	}
	return notFound
}

func findVolumeDeviceByName(name string, volumeDevices []corev1.VolumeDevice) int {
	for idx, volumeDevice := range volumeDevices {
		if volumeDevice.Name == name {
			return idx
		}
	}
	return notFound
}

func findVolumeMountsByMountPath(path string, volumeMounts []corev1.VolumeMount) int {
	for idx, volumeMount := range volumeMounts {
		if volumeMount.MountPath == path {
			return idx
		}
	}
	return notFound
}
