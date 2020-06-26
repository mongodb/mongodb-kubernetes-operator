package podtemplatespec

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPodTemplateSpec(t *testing.T) {
	p := New(
		WithVolume(corev1.Volume{
			Name: "vol-1",
		}),
		WithVolume(corev1.Volume{
			Name: "vol-2",
		}),
		WithFsGroup(100),
		WithImagePullSecrets("pull-secrets"),
		WithInitContainerByIndex(0, container.Apply(
			container.WithName("init-container-0"),
			container.WithImage("init-image"),
		)),
		WithContainerByIndex(0, container.Apply(
			container.WithName("container-0"),
			container.WithImage("image"),
		)),
		WithContainerByIndex(1, container.Apply(
			container.WithName("container-1"),
			container.WithImage("image"),
		)),
	)

	assert.Len(t, p.Spec.Volumes, 2)
	assert.Equal(t, p.Spec.Volumes[0].Name, "vol-1")
	assert.Equal(t, p.Spec.Volumes[1].Name, "vol-2")

	expected := int64(100)
	assert.Equal(t, &expected, p.Spec.SecurityContext.FSGroup)

	assert.Len(t, p.Spec.ImagePullSecrets, 1)
	assert.Equal(t, "pull-secrets", p.Spec.ImagePullSecrets[0].Name)

	assert.Len(t, p.Spec.InitContainers, 1)
	assert.Equal(t, "init-container-0", p.Spec.InitContainers[0].Name)
	assert.Equal(t, "init-image", p.Spec.InitContainers[0].Image)

	assert.Len(t, p.Spec.Containers, 2)
	assert.Equal(t, "container-0", p.Spec.Containers[0].Name)
	assert.Equal(t, "image", p.Spec.Containers[0].Image)
	assert.Equal(t, "container-1", p.Spec.Containers[1].Name)
	assert.Equal(t, "image", p.Spec.Containers[1].Image)
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
