package podtemplatespec

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/merge"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPodTemplateSpec(t *testing.T) {
	volumeMount1 := corev1.VolumeMount{
		Name: "vol-1",
	}
	volumeMount2 := corev1.VolumeMount{
		Name: "vol-2",
	}

	runAsUser := int64(1111)
	runAsGroup := int64(2222)
	fsGroup := int64(3333)

	p := New(
		WithVolume(corev1.Volume{
			Name: "vol-1",
		}),
		WithVolume(corev1.Volume{
			Name: "vol-2",
		}),
		WithSecurityContext(corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &fsGroup,
		}),
		WithImagePullSecrets("pull-secrets"),
		WithInitContainerByIndex(0, container.Apply(
			container.WithName("init-container-0"),
			container.WithImage("init-image"),
			container.WithVolumeMounts([]corev1.VolumeMount{volumeMount1}),
			container.WithSecurityContext(container.DefaultSecurityContext()),
		)),
		WithContainerByIndex(0, container.Apply(
			container.WithName("container-0"),
			container.WithImage("image"),
			container.WithVolumeMounts([]corev1.VolumeMount{volumeMount1}),
			container.WithSecurityContext(container.DefaultSecurityContext()),
		)),
		WithContainerByIndex(1, container.Apply(
			container.WithName("container-1"),
			container.WithImage("image"),
			container.WithSecurityContext(container.DefaultSecurityContext()),
		)),
		WithVolumeMounts("init-container-0", volumeMount2),
		WithVolumeMounts("container-0", volumeMount2),
		WithVolumeMounts("container-1", volumeMount1, volumeMount2),
	)

	assert.Len(t, p.Spec.Volumes, 2)
	assert.Equal(t, p.Spec.Volumes[0].Name, "vol-1")
	assert.Equal(t, p.Spec.Volumes[1].Name, "vol-2")

	expectedRunAsUser := int64(1111)
	expectedRunAsGroup := int64(2222)
	expectedFsGroup := int64(3333)
	assert.Equal(t, &expectedRunAsUser, p.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, &expectedRunAsGroup, p.Spec.SecurityContext.RunAsGroup)
	assert.Equal(t, &expectedFsGroup, p.Spec.SecurityContext.FSGroup)

	assert.Len(t, p.Spec.ImagePullSecrets, 1)
	assert.Equal(t, "pull-secrets", p.Spec.ImagePullSecrets[0].Name)

	assert.Len(t, p.Spec.InitContainers, 1)
	assert.Equal(t, "init-container-0", p.Spec.InitContainers[0].Name)
	assert.Equal(t, "init-image", p.Spec.InitContainers[0].Image)
	assert.Equal(t, []corev1.VolumeMount{volumeMount1, volumeMount2}, p.Spec.InitContainers[0].VolumeMounts)
	assert.Equal(t, container.DefaultSecurityContext(), *p.Spec.InitContainers[0].SecurityContext)

	assert.Len(t, p.Spec.Containers, 2)

	assert.Equal(t, "container-0", p.Spec.Containers[0].Name)
	assert.Equal(t, "image", p.Spec.Containers[0].Image)
	assert.Equal(t, []corev1.VolumeMount{volumeMount1, volumeMount2}, p.Spec.Containers[0].VolumeMounts)
	assert.Equal(t, container.DefaultSecurityContext(), *p.Spec.Containers[0].SecurityContext)

	assert.Equal(t, "container-1", p.Spec.Containers[1].Name)
	assert.Equal(t, "image", p.Spec.Containers[1].Image)
	assert.Equal(t, []corev1.VolumeMount{volumeMount1, volumeMount2}, p.Spec.Containers[1].VolumeMounts)
	assert.Equal(t, container.DefaultSecurityContext(), *p.Spec.Containers[1].SecurityContext)
}

func TestPodTemplateSpec_MultipleEditsToContainer(t *testing.T) {
	p := New(
		WithContainerByIndex(0,
			container.Apply(
				container.WithName("container-0"),
			)),
		WithContainerByIndex(0,
			container.Apply(
				container.WithImage("image"),
			)),
		WithContainerByIndex(0,
			container.Apply(
				container.WithImagePullPolicy(corev1.PullAlways),
			)),
		WithContainer("container-0", container.Apply(
			container.WithCommand([]string{"cmd"}),
		)),
	)

	assert.Len(t, p.Spec.Containers, 1)
	c := p.Spec.Containers[0]
	assert.Equal(t, "container-0", c.Name)
	assert.Equal(t, "image", c.Image)
	assert.Equal(t, corev1.PullAlways, c.ImagePullPolicy)
	assert.Equal(t, "cmd", c.Command[0])
}

func TestMerge(t *testing.T) {
	defaultSpec := getDefaultPodSpec()
	customSpec := getCustomPodSpec()

	mergedSpec := merge.PodTemplateSpecs(defaultSpec, customSpec)

	initContainerDefault := getDefaultContainer()
	initContainerDefault.Name = "init-container-default"

	initContainerCustom := getCustomContainer()
	initContainerCustom.Name = "init-container-custom"

	expected := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-default-name",
			Namespace: "my-default-namespace",
			Labels: map[string]string{
				"app":    "operator",
				"custom": "some",
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node-0": "node-0",
				"node-1": "node-1",
			},
			ServiceAccountName:            "my-service-account-override",
			TerminationGracePeriodSeconds: int64Ref(11),
			ActiveDeadlineSeconds:         int64Ref(10),
			NodeName:                      "my-node-name",
			RestartPolicy:                 corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				getDefaultContainer(),
				getCustomContainer(),
			},
			InitContainers: []corev1.Container{
				initContainerCustom,
				initContainerDefault,
			},
			Volumes:  []corev1.Volume{},
			Affinity: affinity("zone", "custom"),
		},
	}
	assert.Equal(t, expected.Name, mergedSpec.Name)
	assert.Equal(t, expected.Namespace, mergedSpec.Namespace)
	assert.Equal(t, expected.Labels["app"], mergedSpec.Labels["app"])
	assert.Equal(t, expected.Labels["custom"], mergedSpec.Labels["custom"])
	assert.Equal(t, expected.Spec.NodeSelector["node-0"], mergedSpec.Spec.NodeSelector["node-0"])
	assert.Equal(t, expected.Spec.NodeSelector["node-1"], mergedSpec.Spec.NodeSelector["node-1"])
	assert.Equal(t, expected.Spec.ServiceAccountName, mergedSpec.Spec.ServiceAccountName)
	assert.Equal(t, expected.Spec.TerminationGracePeriodSeconds, mergedSpec.Spec.TerminationGracePeriodSeconds)
	assert.Equal(t, expected.Spec.ActiveDeadlineSeconds, mergedSpec.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, expected.Spec.NodeName, mergedSpec.Spec.NodeName)
	assert.Equal(t, expected.Spec.RestartPolicy, mergedSpec.Spec.RestartPolicy)
	assert.Equal(t, expected.Spec.Volumes, mergedSpec.Spec.Volumes)
	assert.Equal(t, expected.Spec.Affinity.PodAntiAffinity, mergedSpec.Spec.Affinity.PodAntiAffinity)
	assert.Equal(t, expected.Spec.Affinity.PodAffinity, mergedSpec.Spec.Affinity.PodAffinity)
	assert.Equal(t, expected.Spec.Affinity.NodeAffinity, mergedSpec.Spec.Affinity.NodeAffinity)
	assert.Equal(t, expected.Spec.Containers, mergedSpec.Spec.Containers)
	assert.Equal(t, expected.Spec.InitContainers, mergedSpec.Spec.InitContainers)
}

func TestMergeFromEmpty(t *testing.T) {
	defaultPodSpec := corev1.PodTemplateSpec{}
	customPodSpecTemplate := getCustomPodSpec()

	mergedPodTemplateSpec := merge.PodTemplateSpecs(defaultPodSpec, customPodSpecTemplate)
	assert.Equal(t, customPodSpecTemplate, mergedPodTemplateSpec)
}

func TestMergeWithEmpty(t *testing.T) {
	defaultPodSpec := getDefaultPodSpec()
	customPodSpecTemplate := corev1.PodTemplateSpec{}

	mergedPodTemplateSpec := merge.PodTemplateSpecs(defaultPodSpec, customPodSpecTemplate)

	assert.Equal(t, defaultPodSpec, mergedPodTemplateSpec)
}

func TestMultipleMerges(t *testing.T) {
	defaultPodSpec := getDefaultPodSpec()
	customPodSpecTemplate := getCustomPodSpec()

	referenceSpec := merge.PodTemplateSpecs(defaultPodSpec, customPodSpecTemplate)

	mergedSpec := defaultPodSpec

	// multiple merges must give the same result
	for i := 0; i < 3; i++ {
		mergedSpec := merge.PodTemplateSpecs(mergedSpec, customPodSpecTemplate)
		assert.Equal(t, referenceSpec, mergedSpec)
	}
}

func TestMergeEnvironmentVariables(t *testing.T) {
	otherDefaultContainer := getDefaultContainer()
	otherDefaultContainer.Env = append(otherDefaultContainer.Env, corev1.EnvVar{
		Name:  "name1",
		Value: "val1",
	})

	overrideOtherDefaultContainer := getDefaultContainer()
	overrideOtherDefaultContainer.Env = append(overrideOtherDefaultContainer.Env, corev1.EnvVar{
		Name:  "name2",
		Value: "val2",
	})
	overrideOtherDefaultContainer.Env = append(overrideOtherDefaultContainer.Env, corev1.EnvVar{
		Name:  "name1",
		Value: "changedValue",
	})

	defaultSpec := getDefaultPodSpec()
	defaultSpec.Spec.Containers = []corev1.Container{otherDefaultContainer}

	customSpec := getCustomPodSpec()
	customSpec.Spec.Containers = []corev1.Container{overrideOtherDefaultContainer}

	mergedSpec := merge.PodTemplateSpecs(defaultSpec, customSpec)

	mergedContainer := mergedSpec.Spec.Containers[0]

	assert.Len(t, mergedContainer.Env, 2)
	assert.Equal(t, mergedContainer.Env[0].Name, "name1")
	assert.Equal(t, mergedContainer.Env[0].Value, "changedValue")
	assert.Equal(t, mergedContainer.Env[1].Name, "name2")
	assert.Equal(t, mergedContainer.Env[1].Value, "val2")
}

func TestMergeTolerations(t *testing.T) {
	tests := []struct {
		name                string
		defaultTolerations  []corev1.Toleration
		overrideTolerations []corev1.Toleration
		expectedTolerations []corev1.Toleration
	}{
		{
			// In case the calling code specifies default tolerations,
			// they should be kept when there are no overrides.
			name: "Overriding with nil tolerations",
			defaultTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
			overrideTolerations: nil,
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
		},
		{
			// If the override is specifying an empty list of tolerations,
			// they should replace default tolerations.
			name: "Overriding with empty tolerations",
			defaultTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
			},
			overrideTolerations: []corev1.Toleration{},
			expectedTolerations: []corev1.Toleration{},
		},
		{
			// Overriding toleration should replace a nil original toleration.
			name:               "Overriding when default toleration is nil",
			defaultTolerations: nil,
			overrideTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
		},
		{
			// Overriding toleration should replace any original toleration.
			name: "Overriding when original toleration is not nil",
			defaultTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value3",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value4",
					Operator: corev1.TolerationOpExists,
				},
			},
			overrideTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
			expectedTolerations: []corev1.Toleration{
				{
					Key:      "key1",
					Value:    "value1",
					Operator: corev1.TolerationOpEqual,
				},
				{
					Key:      "key1",
					Value:    "value2",
					Operator: corev1.TolerationOpExists,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultSpec := getDefaultPodSpec()
			defaultSpec.Spec.Tolerations = tt.defaultTolerations
			overrideSpec := getDefaultPodSpec()
			overrideSpec.Spec.Tolerations = tt.overrideTolerations

			mergedSpec := merge.PodTemplateSpecs(defaultSpec, overrideSpec)
			assert.Equal(t, tt.expectedTolerations, mergedSpec.Spec.Tolerations)
		})
	}
}

func TestMergeContainer(t *testing.T) {
	vol0 := corev1.VolumeMount{Name: "container-0.volume-mount-0"}
	sideCarVol := corev1.VolumeMount{Name: "container-1.volume-mount-0"}

	anotherVol := corev1.VolumeMount{Name: "another-mount"}

	overrideDefaultContainer := corev1.Container{Name: "container-0"}
	overrideDefaultContainer.Image = "overridden"
	overrideDefaultContainer.ReadinessProbe = &corev1.Probe{PeriodSeconds: 20}

	otherDefaultContainer := getDefaultContainer()
	otherDefaultContainer.Name = "default-side-car"
	otherDefaultContainer.VolumeMounts = []corev1.VolumeMount{sideCarVol}

	overrideOtherDefaultContainer := otherDefaultContainer
	overrideOtherDefaultContainer.Env = []corev1.EnvVar{{Name: "env_var", Value: "xxx"}}
	overrideOtherDefaultContainer.VolumeMounts = []corev1.VolumeMount{anotherVol}

	defaultSpec := getDefaultPodSpec()
	defaultSpec.Spec.Containers = []corev1.Container{getDefaultContainer(), otherDefaultContainer}

	customSpec := getCustomPodSpec()
	customSpec.Spec.Containers = []corev1.Container{getCustomContainer(), overrideDefaultContainer, overrideOtherDefaultContainer}

	mergedSpec := merge.PodTemplateSpecs(defaultSpec, customSpec)

	assert.Len(t, mergedSpec.Spec.Containers, 3)
	assert.Equal(t, getCustomContainer(), mergedSpec.Spec.Containers[1])

	firstExpected := corev1.Container{
		Name:         "container-0",
		VolumeMounts: []corev1.VolumeMount{vol0},
		Image:        "overridden",
		Command:      []string{},
		Args:         []string{},
		Ports:        []corev1.ContainerPort{},
		ReadinessProbe: &corev1.Probe{
			// only "periodSeconds" was overwritten - other fields stayed untouched
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/foo"},
			},
			PeriodSeconds: 20,
		},
	}
	assert.Equal(t, firstExpected, mergedSpec.Spec.Containers[0])

	secondExpected := corev1.Container{
		Name:         "default-side-car",
		Image:        "image-0",
		VolumeMounts: []corev1.VolumeMount{anotherVol, sideCarVol},
		Command:      []string{},
		Args:         []string{},
		Ports:        []corev1.ContainerPort{},
		Env: []corev1.EnvVar{
			{
				Name:  "env_var",
				Value: "xxx",
			},
		},
		ReadinessProbe: otherDefaultContainer.ReadinessProbe,
	}
	assert.Equal(t, secondExpected, mergedSpec.Spec.Containers[2])
}

func TestMergeVolumes_DoesNotAddDuplicatesWithSameName(t *testing.T) {
	defaultPodSpec := getDefaultPodSpec()
	defaultPodSpec.Spec.Volumes = append(defaultPodSpec.Spec.Volumes, corev1.Volume{
		Name: "new-volume",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "old-host-path",
			},
		},
	})
	defaultPodSpec.Spec.Volumes = append(defaultPodSpec.Spec.Volumes, corev1.Volume{
		Name: "new-volume-2",
	})

	overridePodSpec := getDefaultPodSpec()
	overridePodSpec.Spec.Volumes = append(overridePodSpec.Spec.Volumes, corev1.Volume{
		Name: "new-volume",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "updated-host-path",
			},
		},
	})
	overridePodSpec.Spec.Volumes = append(overridePodSpec.Spec.Volumes, corev1.Volume{
		Name: "new-volume-3",
	})

	mergedPodSpecTemplate := merge.PodTemplateSpecs(defaultPodSpec, overridePodSpec)

	assert.Len(t, mergedPodSpecTemplate.Spec.Volumes, 3)
	assert.Equal(t, "new-volume", mergedPodSpecTemplate.Spec.Volumes[0].Name)
	assert.Equal(t, "updated-host-path", mergedPodSpecTemplate.Spec.Volumes[0].VolumeSource.HostPath.Path)
	assert.Equal(t, "new-volume-2", mergedPodSpecTemplate.Spec.Volumes[1].Name)
	assert.Equal(t, "new-volume-3", mergedPodSpecTemplate.Spec.Volumes[2].Name)
}

func TestAddVolumes(t *testing.T) {
	volumeModification := WithVolume(corev1.Volume{
		Name: "new-volume",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "old-host-path",
			},
		}},
	)

	toAddVolumes := []corev1.Volume{
		{
			Name: "new-volume",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "new-host-path",
				},
			},
		},
		{
			Name: "new-volume-2",
		},
	}

	volumesModification := WithVolumes(toAddVolumes)

	p := New(volumeModification, volumesModification)
	assert.Len(t, p.Spec.Volumes, 2)
	assert.Equal(t, "new-volume", p.Spec.Volumes[0].Name)
	assert.Equal(t, "new-volume-2", p.Spec.Volumes[1].Name)
	assert.Equal(t, "new-host-path", p.Spec.Volumes[0].VolumeSource.HostPath.Path)
}

func int64Ref(i int64) *int64 {
	return &i
}

func getDefaultPodSpec() corev1.PodTemplateSpec {
	initContainer := getDefaultContainer()
	initContainer.Name = "init-container-default"

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-default-name",
			Namespace:   "my-default-namespace",
			Labels:      map[string]string{"app": "operator"},
			Annotations: map[string]string{},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node-0": "node-0",
			},
			ServiceAccountName:            "my-default-service-account",
			TerminationGracePeriodSeconds: int64Ref(12),
			ActiveDeadlineSeconds:         int64Ref(10),
			Containers:                    []corev1.Container{getDefaultContainer()},
			InitContainers:                []corev1.Container{initContainer},
			Affinity:                      affinity("hostname", "default"),
			Volumes:                       []corev1.Volume{},
		},
	}
}

func getCustomPodSpec() corev1.PodTemplateSpec {
	initContainer := getCustomContainer()
	initContainer.Name = "init-container-custom"

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"custom": "some"},
			Annotations: map[string]string{},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"node-1": "node-1",
			},
			ServiceAccountName:            "my-service-account-override",
			TerminationGracePeriodSeconds: int64Ref(11),
			NodeName:                      "my-node-name",
			RestartPolicy:                 corev1.RestartPolicyAlways,
			Containers:                    []corev1.Container{getCustomContainer()},
			InitContainers:                []corev1.Container{initContainer},
			Affinity:                      affinity("zone", "custom"),
			Volumes:                       []corev1.Volume{},
		},
	}
}

func affinity(antiAffinityKey, nodeAffinityKey string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					TopologyKey: antiAffinityKey,
				},
			}},
		},
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchFields: []corev1.NodeSelectorRequirement{{
					Key: nodeAffinityKey,
				}},
			}}},
		},
	}
}

func getDefaultContainer() corev1.Container {
	return corev1.Container{
		Args:    []string{},
		Command: []string{},
		Ports:   []corev1.ContainerPort{},
		Name:    "container-0",
		Image:   "image-0",
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
				Path: "/foo",
			}},
			PeriodSeconds: 10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name: "container-0.volume-mount-0",
			},
		},
	}
}

func getCustomContainer() corev1.Container {
	return corev1.Container{
		Name:  "container-1",
		Image: "image-1",
	}
}
