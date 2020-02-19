package sts

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestNamespace = "test-ns"
	TestName      = "test-name"
)

func TestGetContainerIndexByName(t *testing.T) {
	containers := []corev1.Container{{
		Name: "container-0",
	},
		{
			Name: "container-1",
		},
		{
			Name: "container-2",
		}}

	sts := defaultStatefulSetBuilder().SetPodTemplateSpec(podTemplateWithContainers(containers)).Build()
	idx, err := getContainerIndexByName(sts, "container-0")

	assert.NoError(t, err)
	assert.NotEqual(t, -1, idx)
	assert.Equal(t, 0, idx)

	idx, err = getContainerIndexByName(sts, "container-1")

	assert.NoError(t, err)
	assert.NotEqual(t, -1, idx)
	assert.Equal(t, 1, idx)

	idx, err = getContainerIndexByName(sts, "container-2")

	assert.NoError(t, err)
	assert.NotEqual(t, -1, idx)
	assert.Equal(t, 2, idx)

	idx, err = getContainerIndexByName(sts, "doesnt-exist")

	assert.Error(t, err)
	assert.Equal(t, -1, idx)
}

func TestMountVolume(t *testing.T) {
	vmd := VolumeMountData{
		MountPath:  "mount-path",
		Name:       "mount-name",
		SourceType: corev1.ConfigMapVolumeSource{},
		SourceName: "config-map",
	}

	sts := defaultStatefulSetBuilder().SetPodTemplateSpec(podTemplateWithContainers([]corev1.Container{{Name: "container-name"}})).Build()

	err := MountVolume(&sts, vmd, "container-name")

	// assert container was correctly updated with the volumes
	assert.NoError(t, err, "volume should successfully mount when the container exists")
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts, 1, "volume mount should have been added to the container in the stateful set")
	assert.Equal(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, "mount-name")
	assert.Equal(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath, "mount-path")

	// assert the volumes were added to the podspec template
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, sts.Spec.Template.Spec.Volumes[0].Name, "mount-name")
	assert.NotNil(t, sts.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap, "volume should have been configured from a config map source")
	assert.Nil(t, sts.Spec.Template.Spec.Volumes[0].VolumeSource.Secret, "volume should not have been configured from a secret source")

	sts = defaultStatefulSetBuilder().SetPodTemplateSpec(podTemplateWithContainers([]corev1.Container{{Name: "container-0"}, {Name: "container-1"}})).Build()

	err = MountVolume(&sts, vmd, "container-0")
	assert.NoError(t, err)

	secretVmd := VolumeMountData{
		MountPath:  "mount-path-secret",
		Name:       "mount-name-secret",
		SourceType: corev1.SecretVolumeSource{},
		SourceName: "secret",
	}

	err = MountVolume(&sts, secretVmd, "container-1")
	assert.NoError(t, err, "volume should successfully mount when the container exists")
	assert.Len(t, sts.Spec.Template.Spec.Containers[1].VolumeMounts, 1, "volume mount should have been added to the container in the stateful set")
	assert.Equal(t, sts.Spec.Template.Spec.Containers[1].VolumeMounts[0].Name, "mount-name-secret")
	assert.Equal(t, sts.Spec.Template.Spec.Containers[1].VolumeMounts[0].MountPath, "mount-path-secret")

	assert.Len(t, sts.Spec.Template.Spec.Volumes, 2)
	assert.Equal(t, sts.Spec.Template.Spec.Volumes[1].Name, "mount-name-secret")
	assert.Nil(t, sts.Spec.Template.Spec.Volumes[1].VolumeSource.ConfigMap, "volume should not have been configured from a config map source")
	assert.NotNil(t, sts.Spec.Template.Spec.Volumes[1].VolumeSource.Secret, "volume should have been configured from a secret source")

	invalidVmd := VolumeMountData{
		SourceType: corev1.Secret{}, // an invalid type
	}

	assert.Panics(t, func() {
		_ = MountVolume(&sts, invalidVmd, "container-1")
	})
}

func TestAddVolumeClaimTemplates(t *testing.T) {
	sts := defaultStatefulSetBuilder().SetPodTemplateSpec(podTemplateWithContainers([]corev1.Container{{Name: "container-name"}})).Build()
	claim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claim-0",
		},
	}
	mount := corev1.VolumeMount{
		Name: "mount-0",
	}
	err := AddVolumeClaimTemplates(&sts, "container-name", []corev1.PersistentVolumeClaim{claim}, []corev1.VolumeMount{mount})

	assert.NoError(t, err)
	assert.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	assert.Equal(t, sts.Spec.VolumeClaimTemplates[0].Name, "claim-0")
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name, "mount-0")
}

func defaultStatefulSetBuilder() builder {
	return Builder().
		SetName(TestName).
		SetNamespace(TestNamespace).
		SetServiceName(fmt.Sprintf("%s-svc", TestName)).
		SetLabels(map[string]string{})
}

func podTemplateWithContainers(containers []corev1.Container) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: containers,
		},
	}
}
