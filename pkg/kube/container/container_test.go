package container

import (
	"fmt"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/lifecycle"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestContainer(t *testing.T) {
	c := New(
		WithName("name"),
		WithImage("image"),
		WithImagePullPolicy(corev1.PullAlways),
		WithPorts([]corev1.ContainerPort{{Name: "port-1", ContainerPort: int32(1000)}}),
		WithSecurityContext(corev1.SecurityContext{
			RunAsGroup:   int64Ref(100),
			RunAsNonRoot: boolRef(true),
		}),
		WithLifecycle(lifecycle.Apply(
			lifecycle.WithPrestopCommand([]string{"pre-stop-command"}),
		)),
		WithReadinessProbe(probes.Apply(
			probes.WithExecCommand([]string{"exec"}),
			probes.WithFailureThreshold(10),
			probes.WithPeriodSeconds(5),
		)),
		WithLivenessProbe(probes.Apply(
			probes.WithExecCommand([]string{"liveness-exec"}),
			probes.WithFailureThreshold(15),
			probes.WithPeriodSeconds(10),
		)),
		WithStartupProbe(
			probes.Apply(
				probes.WithExecCommand([]string{"startup-exec"}),
				probes.WithFailureThreshold(20),
				probes.WithPeriodSeconds(30),
			),
		),
		WithResourceRequirements(resourcerequirements.Defaults()),
		WithCommand([]string{"container-cmd"}),
		WithEnvs(
			[]corev1.EnvVar{
				{
					Name:  "env-1",
					Value: "env-1-value",
				},
			}...,
		),
	)

	assert.Equal(t, "name", c.Name)
	assert.Equal(t, "image", c.Image)
	assert.Equal(t, corev1.PullAlways, c.ImagePullPolicy)

	assert.Len(t, c.Ports, 1)
	assert.Equal(t, int32(1000), c.Ports[0].ContainerPort)
	assert.Equal(t, "port-1", c.Ports[0].Name)

	securityContext := c.SecurityContext
	assert.Equal(t, int64Ref(100), securityContext.RunAsGroup)
	assert.Equal(t, boolRef(true), securityContext.RunAsNonRoot)

	readinessProbe := c.ReadinessProbe
	assert.Equal(t, int32(10), readinessProbe.FailureThreshold)
	assert.Equal(t, int32(5), readinessProbe.PeriodSeconds)
	assert.Equal(t, "exec", readinessProbe.Exec.Command[0])

	liveNessProbe := c.LivenessProbe
	assert.Equal(t, int32(15), liveNessProbe.FailureThreshold)
	assert.Equal(t, int32(10), liveNessProbe.PeriodSeconds)
	assert.Equal(t, "liveness-exec", liveNessProbe.Exec.Command[0])

	startupProbe := c.StartupProbe
	assert.Equal(t, int32(20), startupProbe.FailureThreshold)
	assert.Equal(t, int32(30), startupProbe.PeriodSeconds)
	assert.Equal(t, "startup-exec", startupProbe.Exec.Command[0])

	assert.Equal(t, c.Resources, resourcerequirements.Defaults())

	assert.Len(t, c.Command, 1)
	assert.Equal(t, "container-cmd", c.Command[0])

	lifeCycle := c.Lifecycle
	assert.NotNil(t, lifeCycle)
	assert.NotNil(t, lifeCycle.PreStop)
	assert.NotNil(t, lifeCycle.PreStop.Exec)
	assert.Equal(t, "pre-stop-command", lifeCycle.PreStop.Exec.Command[0])

	assert.Len(t, c.Env, 1)
	assert.Equal(t, "env-1", c.Env[0].Name)
	assert.Equal(t, "env-1-value", c.Env[0].Value)
}

func TestMergeEnvs(t *testing.T) {
	existing := []corev1.EnvVar{
		{
			Name:  "C_env",
			Value: "C_value",
		},
		{
			Name:  "B_env",
			Value: "B_value",
		},
		{
			Name:  "A_env",
			Value: "A_value",
		},
		{
			Name: "F_env",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "f_key",
				},
			},
		},
	}

	desired := []corev1.EnvVar{
		{
			Name:  "D_env",
			Value: "D_value",
		},
		{
			Name:  "E_env",
			Value: "E_value",
		},
		{
			Name:  "C_env",
			Value: "C_value_new",
		},
		{
			Name:  "B_env",
			Value: "B_value_new",
		},
		{
			Name:  "A_env",
			Value: "A_value",
		},
	}

	merged := envvar.MergeWithOverride(existing, desired) // nolint:forbidigo

	t.Run("EnvVars should be sorted", func(t *testing.T) {
		assert.Equal(t, "A_env", merged[0].Name)
		assert.Equal(t, "B_env", merged[1].Name)
		assert.Equal(t, "C_env", merged[2].Name)
		assert.Equal(t, "D_env", merged[3].Name)
		assert.Equal(t, "E_env", merged[4].Name)
		assert.Equal(t, "F_env", merged[5].Name)
	})

	t.Run("EnvVars of same name are updated", func(t *testing.T) {
		assert.Equal(t, "B_env", merged[1].Name)
		assert.Equal(t, "B_value_new", merged[1].Value)
	})

	t.Run("Existing EnvVars are not touched", func(t *testing.T) {
		envVar := merged[5]
		assert.NotNil(t, envVar.ValueFrom)
		assert.Equal(t, "f_key", envVar.ValueFrom.SecretKeyRef.Key)
	})
}

func TestWithVolumeMounts(t *testing.T) {
	c := New(
		WithVolumeMounts(
			[]corev1.VolumeMount{
				{
					Name:      "name-0",
					MountPath: "mount-path-0",
					SubPath:   "sub-path-0",
				},
				{
					Name:      "name-1",
					MountPath: "mount-path-1",
					SubPath:   "sub-path-1",
				},
				{
					Name:      "name-2",
					MountPath: "mount-path-2",
					SubPath:   "sub-path-2",
				},
			},
		),
	)

	newVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "name-0",
			MountPath: "mount-path-0",
			SubPath:   "sub-path-0",
		},
		{
			Name:      "name-4",
			MountPath: "mount-path-4",
			SubPath:   "sub-path-4",
		},
		{
			Name:      "name-3",
			MountPath: "mount-path-3",
			SubPath:   "sub-path-3",
		},
	}

	WithVolumeMounts(newVolumeMounts)(&c)

	assert.Len(t, c.VolumeMounts, 5, "duplicates should have been removed")
	for i, v := range c.VolumeMounts {
		assert.Equal(t, fmt.Sprintf("name-%d", i), v.Name, "Volumes should be sorted but were not!")
		assert.Equal(t, fmt.Sprintf("mount-path-%d", i), v.MountPath, "Volumes should be sorted but were not!")
		assert.Equal(t, fmt.Sprintf("sub-path-%d", i), v.SubPath, "Volumes should be sorted but were not!")
	}

}

func boolRef(b bool) *bool {
	return &b
}

func int64Ref(i int64) *int64 {
	return &i
}
