package merge

import (
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeStringSlices(t *testing.T) {
	type args struct {
		original []string
		override []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Does not include duplicate entries",
			args: args{
				original: []string{"a", "b", "c"},
				override: []string{"a", "c"},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "Adds elements from override",
			args: args{
				original: []string{"a", "b", "c"},
				override: []string{"a", "b", "c", "d", "e"},
			},
			want: []string{"a", "b", "c", "d", "e"},
		},
		{
			name: "Doesn't panic with nil input",
			args: args{
				original: nil,
				override: nil,
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringSlices(tt.args.original, tt.args.override); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeStringSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeServices(t *testing.T) {
	type args struct {
		original corev1.ServiceSpec
		override corev1.ServiceSpec
	}
	tests := []struct {
		name string
		args args
		want corev1.ServiceSpec
	}{
		{
			name: "Overrides a few example spec values",
			args: args{
				original: corev1.ServiceSpec{},
				override: corev1.ServiceSpec{
					Type:                     "LoadBalancer",
					ExternalName:             "externalName",
					ExternalTrafficPolicy:    "some-non-existing-policy",
					HealthCheckNodePort:      123,
					PublishNotReadyAddresses: true,
				},
			},
			want: corev1.ServiceSpec{
				Type:                     "LoadBalancer",
				ExternalName:             "externalName",
				ExternalTrafficPolicy:    "some-non-existing-policy",
				HealthCheckNodePort:      123,
				PublishNotReadyAddresses: true,
			},
		},
		{
			name: "Merge labels",
			args: args{
				original: corev1.ServiceSpec{
					Selector: map[string]string{"test1": "true"},
				},
				override: corev1.ServiceSpec{
					Selector: map[string]string{"test2": "true"},
				},
			},
			want: corev1.ServiceSpec{
				Selector: map[string]string{"test1": "true", "test2": "true"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ServiceSpec(tt.args.original, tt.args.override); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeContainer(t *testing.T) {
	defaultQuantity := resource.NewQuantity(int64(10), resource.DecimalExponent)

	defaultContainer := container.New(
		container.WithName("default-container"),
		container.WithCommand([]string{"a", "b", "c"}),
		container.WithImage("default-image"),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithWorkDir("default-work-dir"),
		container.WithArgs([]string{"arg0", "arg1"}),
		container.WithLivenessProbe(probes.Apply(
			probes.WithInitialDelaySeconds(10),
			probes.WithFailureThreshold(20),
			probes.WithExecCommand([]string{"exec", "command", "liveness"}),
		)),
		container.WithReadinessProbe(probes.Apply(
			probes.WithInitialDelaySeconds(20),
			probes.WithFailureThreshold(30),
			probes.WithExecCommand([]string{"exec", "command", "readiness"}),
		)),
		container.WithVolumeDevices([]corev1.VolumeDevice{
			{
				Name:       "name-0",
				DevicePath: "original-path-0",
			},
			{
				Name:       "name-1",
				DevicePath: "original-path-1",
			},
		}),
		container.WithVolumeMounts([]corev1.VolumeMount{
			{
				Name:             "volume-mount-0",
				ReadOnly:         false,
				MountPath:        "original-mount-path",
				SubPath:          "original-sub-path",
				MountPropagation: nil,
				SubPathExpr:      "original-sub-path-expr",
			},
			{
				Name:             "volume-mount-1",
				ReadOnly:         false,
				MountPath:        "original-mount-path-1",
				SubPath:          "original-sub-path-1",
				MountPropagation: nil,
				SubPathExpr:      "original-sub-path-expr-1",
			},
		}),
		container.WithResourceRequirements(
			corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"limit": *defaultQuantity,
				},
			}),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  "env0",
				Value: "val1",
			},
			corev1.EnvVar{
				Name:  "env3",
				Value: "val3",
			}),
	)

	t.Run("Override Fields", func(t *testing.T) {
		overrideQuantity := resource.NewQuantity(int64(15), resource.BinarySI)

		overrideContainer := container.New(
			container.WithName("override-container"),
			container.WithCommand([]string{"d", "f", "e"}),
			container.WithImage("override-image"),
			container.WithWorkDir("override-work-dir"),
			container.WithArgs([]string{"arg3", "arg2"}),
			container.WithLivenessProbe(probes.Apply(
				probes.WithInitialDelaySeconds(15),
				probes.WithExecCommand([]string{"exec", "command", "override"}),
			)),
			container.WithReadinessProbe(probes.Apply(
				probes.WithInitialDelaySeconds(5),
				probes.WithFailureThreshold(6),
				probes.WithExecCommand([]string{"exec", "command", "readiness", "override"}),
			)),
			container.WithVolumeDevices([]corev1.VolumeDevice{
				{
					Name:       "name-0",
					DevicePath: "override-path-0",
				},
				{
					Name:       "name-2",
					DevicePath: "override-path-2",
				},
			}),
			container.WithVolumeMounts([]corev1.VolumeMount{
				{
					Name:             "volume-mount-1",
					ReadOnly:         true,
					MountPath:        "override-mount-path-1",
					SubPath:          "override-sub-path-1",
					MountPropagation: nil,
					SubPathExpr:      "override-sub-path-expr-1",
				},
			}),
			container.WithResourceRequirements(
				corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"limits": *overrideQuantity,
					},
					Requests: corev1.ResourceList{
						"requests": *overrideQuantity,
					},
				}),
			container.WithEnvs(
				corev1.EnvVar{
					Name:  "env0",
					Value: "val2",
				},
				corev1.EnvVar{
					Name:      "env3",
					ValueFrom: &corev1.EnvVarSource{},
				},
			),
		)
		mergedContainer := Container(defaultContainer, overrideContainer)
		assert.Equal(t, overrideContainer.Name, mergedContainer.Name, "Name was overridden, and should be used.")
		assert.Equal(t, []string{"d", "f", "e"}, mergedContainer.Command, "Command specified in the override container overrides the default container.")
		assert.Equal(t, overrideContainer.Image, mergedContainer.Image, "Image was overridden, and should be used.")
		assert.Equal(t, defaultContainer.ImagePullPolicy, mergedContainer.ImagePullPolicy, "No ImagePullPolicy was specified in the override, so the default should be used.")
		assert.Equal(t, overrideContainer.WorkingDir, mergedContainer.WorkingDir)
		assert.Equal(t, []string{"arg3", "arg2"}, mergedContainer.Args, "Args specified in the override container overrides the default container.")

		assert.Equal(t, overrideContainer.Resources, mergedContainer.Resources)

		t.Run("Env are overridden", func(t *testing.T) {
			assert.Len(t, mergedContainer.Env, 2)
			assert.Equal(t, "env0", mergedContainer.Env[0].Name)
			assert.Equal(t, "val2", mergedContainer.Env[0].Value)
			assert.Equal(t, "env3", mergedContainer.Env[1].Name)
			assert.Equal(t, "", mergedContainer.Env[1].Value)
			assert.NotNil(t, mergedContainer.Env[1].ValueFrom)
		})

		t.Run("Probes are overridden", func(t *testing.T) {
			t.Run("Liveness probe", func(t *testing.T) {
				livenessProbe := mergedContainer.LivenessProbe

				assert.NotNil(t, livenessProbe)
				assert.Equal(t, int32(15), livenessProbe.InitialDelaySeconds, "value is specified in override and so should be used.")
				assert.Equal(t, int32(20), livenessProbe.FailureThreshold, "value is not specified in override so the original should be used.")
				assert.Equal(t, []string{"exec", "command", "override"}, livenessProbe.Exec.Command, "value is not specified in override so the original should be used.")
			})
			t.Run("Readiness probe", func(t *testing.T) {
				readinessProbe := mergedContainer.ReadinessProbe
				assert.NotNil(t, readinessProbe)
				assert.Equal(t, int32(5), readinessProbe.InitialDelaySeconds, "value is specified in override and so should be used.")
				assert.Equal(t, int32(6), readinessProbe.FailureThreshold, "value is not specified in override so the original should be used.")
				assert.Equal(t, []string{"exec", "command", "readiness", "override"}, readinessProbe.Exec.Command, "value is not specified in override so the original should be used.")
			})
		})

		t.Run("Volume Devices are overridden", func(t *testing.T) {
			volumeDevices := mergedContainer.VolumeDevices
			assert.Len(t, volumeDevices, 3)
			t.Run("VolumeDevice0 was updated", func(t *testing.T) {
				vd0 := volumeDevices[0]
				assert.Equal(t, "name-0", vd0.Name)
				assert.Equal(t, "override-path-0", vd0.DevicePath)
			})
			t.Run("VolumeDevice1 remained unchanged", func(t *testing.T) {
				vd1 := volumeDevices[1]
				assert.Equal(t, "name-1", vd1.Name)
				assert.Equal(t, "original-path-1", vd1.DevicePath)
			})
			t.Run("VolumeDevice2 was updated", func(t *testing.T) {
				vd2 := volumeDevices[2]
				assert.Equal(t, "name-2", vd2.Name)
				assert.Equal(t, "override-path-2", vd2.DevicePath)
			})
		})

		t.Run("Volume Mounts are overridden", func(t *testing.T) {
			volumeMounts := mergedContainer.VolumeMounts
			assert.Len(t, volumeMounts, 3, "volume mounts can have the same name, the uniqueness is the combination of name, path and subpath")
			t.Run("First VolumeMount is still present", func(t *testing.T) {
				vm0 := volumeMounts[0]
				assert.Equal(t, "volume-mount-0", vm0.Name)
				assert.False(t, vm0.ReadOnly)
				assert.Equal(t, "original-mount-path", vm0.MountPath)
				assert.Equal(t, "original-sub-path", vm0.SubPath)
				assert.Equal(t, "original-sub-path-expr", vm0.SubPathExpr)
			})
			t.Run("Second VolumeMount has merged values", func(t *testing.T) {
				assert.Equal(t, volumeMounts[0], defaultContainer.VolumeMounts[0])
				assert.Equal(t, volumeMounts[1], defaultContainer.VolumeMounts[1])
				assert.Equal(t, volumeMounts[2], overrideContainer.VolumeMounts[0])
			})
		})
	})

	t.Run("No Override Fields", func(t *testing.T) {
		mergedContainer := Container(defaultContainer, corev1.Container{})
		assert.Equal(t, defaultContainer.Name, mergedContainer.Name, "Name was not overridden, and should not be used.")
		assert.Equal(t, defaultContainer.Image, mergedContainer.Image, "Image was not overridden, and should not be used.")
		assert.Equal(t, defaultContainer.ImagePullPolicy, mergedContainer.ImagePullPolicy, "No ImagePullPolicy was specified in the override, so the default should be used.")
		assert.Equal(t, defaultContainer.WorkingDir, mergedContainer.WorkingDir)

		assert.Equal(t, defaultContainer.Resources, mergedContainer.Resources)

		t.Run("No Overriden Env", func(t *testing.T) {
			assert.Len(t, mergedContainer.Env, 2)
			assert.Equal(t, "env0", mergedContainer.Env[0].Name)
			assert.Equal(t, "val1", mergedContainer.Env[0].Value)
			assert.Equal(t, "env3", mergedContainer.Env[1].Name)
			assert.Equal(t, "val3", mergedContainer.Env[1].Value)
			assert.Nil(t, mergedContainer.Env[1].ValueFrom)
		})

		t.Run("Probes are not overridden", func(t *testing.T) {
			t.Run("Liveness probe", func(t *testing.T) {
				livenessProbe := mergedContainer.LivenessProbe

				assert.NotNil(t, livenessProbe)
				assert.Equal(t, int32(10), livenessProbe.InitialDelaySeconds, "value is not specified in override so the original should be used.")
				assert.Equal(t, int32(20), livenessProbe.FailureThreshold, "value is not specified in override so the original should be used.")
				assert.Equal(t, []string{"exec", "command", "liveness"}, livenessProbe.Exec.Command, "value is not specified in override so the original should be used.")
			})
			t.Run("Readiness probe", func(t *testing.T) {
				readinessProbe := mergedContainer.ReadinessProbe
				assert.NotNil(t, readinessProbe)
				assert.Equal(t, int32(20), readinessProbe.InitialDelaySeconds, "value is not specified in override so the original should be used.")
				assert.Equal(t, int32(30), readinessProbe.FailureThreshold, "value is not specified in override so the original should be used.")
				assert.Equal(t, []string{"exec", "command", "readiness"}, readinessProbe.Exec.Command, "value is not specified in override so the original should be used.")
			})
		})

		t.Run("Volume Devices are not overridden", func(t *testing.T) {
			volumeDevices := mergedContainer.VolumeDevices
			assert.Len(t, volumeDevices, 2)
			t.Run("VolumeDevice0 was updated", func(t *testing.T) {
				vd0 := volumeDevices[0]
				assert.Equal(t, "name-0", vd0.Name)
				assert.Equal(t, "original-path-0", vd0.DevicePath)
			})
			t.Run("VolumeDevice1 remained unchanged", func(t *testing.T) {
				vd1 := volumeDevices[1]
				assert.Equal(t, "name-1", vd1.Name)
				assert.Equal(t, "original-path-1", vd1.DevicePath)
			})
		})

		t.Run("Volume Mounts are not overridden", func(t *testing.T) {
			volumeMounts := mergedContainer.VolumeMounts
			assert.Len(t, volumeMounts, 2)
			t.Run("First VolumeMount is still present and unchanged", func(t *testing.T) {
				vm0 := volumeMounts[0]
				assert.Equal(t, "volume-mount-0", vm0.Name)
				assert.False(t, vm0.ReadOnly)
				assert.Equal(t, "original-mount-path", vm0.MountPath)
				assert.Equal(t, "original-sub-path", vm0.SubPath)
				assert.Equal(t, "original-sub-path-expr", vm0.SubPathExpr)
			})
			t.Run("Second VolumeMount is still present and unchanged", func(t *testing.T) {
				vm1 := volumeMounts[1]
				assert.Equal(t, "volume-mount-1", vm1.Name)
				assert.False(t, vm1.ReadOnly)
				assert.Equal(t, "original-mount-path-1", vm1.MountPath)
				assert.Equal(t, "original-sub-path-1", vm1.SubPath)
				assert.Equal(t, "original-sub-path-expr-1", vm1.SubPathExpr)
			})
		})

	})
}

func TestMergeContainerPort(t *testing.T) {
	original := corev1.ContainerPort{
		Name:          "original-port",
		HostPort:      10,
		ContainerPort: 10,
		Protocol:      corev1.ProtocolTCP,
		HostIP:        "4.3.2.1",
	}

	t.Run("Override Fields", func(t *testing.T) {
		override := corev1.ContainerPort{
			Name:          "override-port",
			HostPort:      1,
			ContainerPort: 5,
			Protocol:      corev1.ProtocolUDP,
			HostIP:        "1.2.3.4",
		}
		mergedPort := ContainerPorts(original, override)

		assert.Equal(t, override.Name, mergedPort.Name)
		assert.Equal(t, override.HostPort, mergedPort.HostPort)
		assert.Equal(t, override.ContainerPort, mergedPort.ContainerPort)
		assert.Equal(t, override.HostIP, mergedPort.HostIP)
		assert.Equal(t, override.ContainerPort, mergedPort.ContainerPort)

	})

	t.Run("No Override Fields", func(t *testing.T) {
		mergedPort := ContainerPorts(original, corev1.ContainerPort{})
		assert.Equal(t, original.Name, mergedPort.Name)
		assert.Equal(t, original.HostPort, mergedPort.HostPort)
		assert.Equal(t, original.ContainerPort, mergedPort.ContainerPort)
		assert.Equal(t, original.HostIP, mergedPort.HostIP)
		assert.Equal(t, original.ContainerPort, mergedPort.ContainerPort)
	})
}

func TestMergeVolumeMount(t *testing.T) {
	hostToContainer := corev1.MountPropagationHostToContainer
	hostToContainerRef := &hostToContainer
	original := corev1.VolumeMount{
		Name:             "override-name",
		ReadOnly:         true,
		MountPath:        "override-mount-path",
		SubPath:          "override-sub-path",
		MountPropagation: hostToContainerRef,
		SubPathExpr:      "override-sub-path-expr",
	}

	t.Run("With Override", func(t *testing.T) {
		bidirectional := corev1.MountPropagationBidirectional
		bidirectionalRef := &bidirectional
		override := corev1.VolumeMount{
			Name:             "override-name",
			ReadOnly:         true,
			MountPath:        "override-mount-path",
			SubPath:          "override-sub-path",
			MountPropagation: bidirectionalRef,
			SubPathExpr:      "override-sub-path-expr",
		}
		mergedVolumeMount := VolumeMount(original, override)

		assert.Equal(t, override.Name, mergedVolumeMount.Name)
		assert.Equal(t, override.ReadOnly, mergedVolumeMount.ReadOnly)
		assert.Equal(t, override.MountPath, mergedVolumeMount.MountPath)
		assert.Equal(t, override.MountPropagation, mergedVolumeMount.MountPropagation)
		assert.Equal(t, override.SubPathExpr, mergedVolumeMount.SubPathExpr)
	})

	t.Run("No Override", func(t *testing.T) {
		mergedVolumeMount := VolumeMount(original, corev1.VolumeMount{})

		assert.Equal(t, original.Name, mergedVolumeMount.Name)
		assert.Equal(t, original.ReadOnly, mergedVolumeMount.ReadOnly)
		assert.Equal(t, original.MountPath, mergedVolumeMount.MountPath)
		assert.Equal(t, original.MountPropagation, mergedVolumeMount.MountPropagation)
		assert.Equal(t, original.SubPathExpr, mergedVolumeMount.SubPathExpr)
	})
}

func TestContainerPortSlicesByName(t *testing.T) {

	original := []corev1.ContainerPort{
		{
			Name:          "original-port-0",
			HostPort:      10,
			ContainerPort: 10,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.4",
		},
		{
			Name:          "original-port-1",
			HostPort:      20,
			ContainerPort: 20,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.5",
		},
		{
			Name:          "original-port-2",
			HostPort:      30,
			ContainerPort: 30,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.6",
		},
	}

	override := []corev1.ContainerPort{
		{
			Name:          "original-port-0",
			HostPort:      50,
			ContainerPort: 50,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.10",
		},
		{
			Name:          "original-port-1",
			HostPort:      60,
			ContainerPort: 60,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.50",
		},
		{
			Name:          "original-port-3",
			HostPort:      40,
			ContainerPort: 40,
			Protocol:      corev1.ProtocolTCP,
			HostIP:        "1.2.3.6",
		},
	}

	merged := ContainerPortSlicesByName(original, override)

	assert.Len(t, merged, 4, "There are 4 distinct names between the two slices.")

	t.Run("Test Port 0", func(t *testing.T) {
		assert.Equal(t, "original-port-0", merged[0].Name, "The name should remain unchanged")
		assert.Equal(t, int32(50), merged[0].HostPort, "The HostPort should have been overridden")
		assert.Equal(t, int32(50), merged[0].ContainerPort, "The ContainerPort should have been overridden")
		assert.Equal(t, "1.2.3.10", merged[0].HostIP, "The HostIP should have been overridden")
		assert.Equal(t, corev1.ProtocolTCP, merged[0].Protocol, "The Protocol should remain unchanged")
	})
	t.Run("Test Port 1", func(t *testing.T) {
		assert.Equal(t, "original-port-1", merged[1].Name, "The name should remain unchanged")
		assert.Equal(t, int32(60), merged[1].HostPort, "The HostPort should have been overridden")
		assert.Equal(t, int32(60), merged[1].ContainerPort, "The ContainerPort should have been overridden")
		assert.Equal(t, "1.2.3.50", merged[1].HostIP, "The HostIP should have been overridden")
		assert.Equal(t, corev1.ProtocolTCP, merged[1].Protocol, "The Protocol should remain unchanged")
	})
	t.Run("Test Port 2", func(t *testing.T) {
		assert.Equal(t, "original-port-2", merged[2].Name, "The name should remain unchanged")
		assert.Equal(t, int32(30), merged[2].HostPort, "The HostPort should remain unchanged")
		assert.Equal(t, int32(30), merged[2].ContainerPort, "The ContainerPort should remain unchanged")
		assert.Equal(t, "1.2.3.6", merged[2].HostIP, "The HostIP should remain unchanged")
		assert.Equal(t, corev1.ProtocolTCP, merged[2].Protocol, "The Protocol should remain unchanged")
	})
	t.Run("Test Port 3", func(t *testing.T) {
		assert.Equal(t, "original-port-3", merged[3].Name, "The name should remain unchanged")
		assert.Equal(t, int32(40), merged[3].HostPort, "The HostPort should have been overridden")
		assert.Equal(t, int32(40), merged[3].ContainerPort, "The ContainerPort should have been overridden")
		assert.Equal(t, "1.2.3.6", merged[3].HostIP, "The HostIP should have been overridden")
		assert.Equal(t, corev1.ProtocolTCP, merged[3].Protocol, "The Protocol should remain unchanged")
	})

}

func TestMergeSecurityContext(t *testing.T) {
	privileged := true
	windowsRunAsUserName := "username"
	runAsGroup := int64(4)
	original := &corev1.SecurityContext{
		Capabilities: nil,
		Privileged:   &privileged,
		WindowsOptions: &corev1.WindowsSecurityContextOptions{
			RunAsUserName: &windowsRunAsUserName,
		},
		RunAsGroup: &runAsGroup,
	}

	runAsGroup = int64(6)
	override := &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{
				"123",
				"456",
			},
		},
		Privileged: &privileged,
		WindowsOptions: &corev1.WindowsSecurityContextOptions{
			RunAsUserName: &windowsRunAsUserName,
		},
		RunAsGroup: &runAsGroup,
	}

	merged := SecurityContext(original, override)

	assert.Equal(t, int64(6), *merged.RunAsGroup)
	assert.Equal(t, "username", *merged.WindowsOptions.RunAsUserName)
	assert.Equal(t, override.Capabilities, merged.Capabilities)
	assert.True(t, *override.Privileged)
}

func TestMergeVolumesSecret(t *testing.T) {
	permission := int32(416)
	vol0 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "Secret-name"}}}}
	vol1 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{DefaultMode: &permission}}}}
	mergedVolumes := Volumes(vol0, vol1)
	assert.Len(t, mergedVolumes, 1)
	volume := mergedVolumes[0]
	assert.Equal(t, "volume", volume.Name)
	assert.Equal(t, corev1.SecretVolumeSource{SecretName: "Secret-name", DefaultMode: &permission}, *volume.Secret)
}

func TestMergeNonNilValueNotFilledByOperator(t *testing.T) {
	// Tests that providing a custom volume with a volume source
	// That the operator does not manage overwrites the original
	vol0 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "Secret-name"}}}}
	vol1 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{GCEPersistentDisk: &corev1.GCEPersistentDiskVolumeSource{}}}}
	mergedVolumes := Volumes(vol0, vol1)
	assert.Len(t, mergedVolumes, 1)
	volume := mergedVolumes[0]
	assert.Equal(t, "volume", volume.Name)
	assert.Equal(t, corev1.GCEPersistentDiskVolumeSource{}, *volume.GCEPersistentDisk)
	assert.Nil(t, volume.Secret)
}

func TestMergeNonNilValueFilledByOperatorButDifferent(t *testing.T) {
	// Tests that providing a custom volume with a volume source
	// That the operator does manage, but different from the one
	// That already exists, overwrites the original
	vol0 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "Secret-name"}}}}
	vol1 := []corev1.Volume{{Name: "volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	mergedVolumes := Volumes(vol0, vol1)
	assert.Len(t, mergedVolumes, 1)
	volume := mergedVolumes[0]
	assert.Equal(t, "volume", volume.Name)
	assert.Equal(t, corev1.EmptyDirVolumeSource{}, *volume.EmptyDir)
	assert.Nil(t, volume.Secret)
}

func TestMergeVolumeAddVolume(t *testing.T) {
	vol0 := []corev1.Volume{{Name: "volume0", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{}}}}
	vol1 := []corev1.Volume{{Name: "volume1", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	mergedVolumes := Volumes(vol0, vol1)
	assert.Len(t, mergedVolumes, 2)
	volume0 := mergedVolumes[0]
	assert.Equal(t, "volume0", volume0.Name)
	assert.Equal(t, corev1.SecretVolumeSource{}, *volume0.Secret)
	volume1 := mergedVolumes[1]
	assert.Equal(t, "volume1", volume1.Name)
	assert.Equal(t, corev1.EmptyDirVolumeSource{}, *volume1.EmptyDir)
}

func TestMergeHostAliases(t *testing.T) {
	ha0 := []corev1.HostAlias{
		{
			IP: "1.2.3.4",
			Hostnames: []string{
				"abc", "def",
			},
		},
		{
			IP: "1.2.3.5",
			Hostnames: []string{
				"abc",
			},
		},
	}

	ha1 := []corev1.HostAlias{
		{
			IP: "1.2.3.4",
			Hostnames: []string{
				"abc", "def", "ghi",
			},
		},
	}

	merged := HostAliases(ha0, ha1)

	assert.Len(t, merged, 2)
	assert.Equal(t, "1.2.3.4", merged[0].IP)
	assert.Equal(t, []string{"abc", "def", "ghi"}, merged[0].Hostnames)
	assert.Equal(t, "1.2.3.5", merged[1].IP)
	assert.Equal(t, []string{"abc"}, merged[1].Hostnames)
}
