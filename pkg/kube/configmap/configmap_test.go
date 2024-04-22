package configmap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type configMapGetter struct {
	cm corev1.ConfigMap
}

func (c configMapGetter) GetConfigMap(ctx context.Context, objectKey client.ObjectKey) (corev1.ConfigMap, error) {
	if c.cm.Name == objectKey.Name && c.cm.Namespace == objectKey.Namespace {
		return c.cm, nil
	}
	return corev1.ConfigMap{}, notFoundError()
}

func newGetter(cm corev1.ConfigMap) Getter {
	return configMapGetter{
		cm: cm,
	}
}

func TestReadKey(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("key1", "value1").
			SetDataField("key2", "value2").
			Build(),
	)

	value, err := ReadKey(ctx, getter, "key1", nsName("namespace", "name"))
	assert.Equal(t, "value1", value)
	assert.NoError(t, err)

	value, err = ReadKey(ctx, getter, "key2", nsName("namespace", "name"))
	assert.Equal(t, "value2", value)
	assert.NoError(t, err)

	_, err = ReadKey(ctx, getter, "key3", nsName("namespace", "name"))
	assert.Error(t, err)
}

func TestReadData(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("key1", "value1").
			SetDataField("key2", "value2").
			Build(),
	)

	data, err := ReadData(ctx, getter, nsName("namespace", "name"))
	assert.NoError(t, err)

	assert.Contains(t, data, "key1")
	assert.Contains(t, data, "key2")

	assert.Equal(t, "value1", data["key1"])
	assert.Equal(t, "value2", data["key2"])
}

func TestReadFileLikeField(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("key1", "value1=1\nvalue2=2").
			Build(),
	)

	data, err := ReadFileLikeField(ctx, getter, nsName("namespace", "name"), "key1", "value1")
	assert.NoError(t, err)

	assert.Equal(t, "1", data)
}

func TestReadFileLikeField_InvalidExternalKey(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("key1", "value1=1\nvalue2=2").
			Build(),
	)

	_, err := ReadFileLikeField(ctx, getter, nsName("namespace", "name"), "key2", "value1")
	assert.Error(t, err)
	assert.Equal(t, "key key2 is not present in ConfigMap namespace/name", err.Error())
}

func TestReadFileLikeField_InvalidInternalKey(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("key1", "value1=1\nvalue2=2").
			Build(),
	)

	_, err := ReadFileLikeField(ctx, getter, nsName("namespace", "name"), "key1", "value3")
	assert.Error(t, err)
	assert.Equal(t, "key value3 is not present in the key1 field of ConfigMap namespace/name", err.Error())
}

type configMapGetUpdater struct {
	cm corev1.ConfigMap
}

func (c configMapGetUpdater) GetConfigMap(ctx context.Context, objectKey client.ObjectKey) (corev1.ConfigMap, error) {
	if c.cm.Name == objectKey.Name && c.cm.Namespace == objectKey.Namespace {
		return c.cm, nil
	}
	return corev1.ConfigMap{}, notFoundError()
}

func (c *configMapGetUpdater) UpdateConfigMap(ctx context.Context, cm corev1.ConfigMap) error {
	c.cm = cm
	return nil
}

func newGetUpdater(cm corev1.ConfigMap) GetUpdater {
	return &configMapGetUpdater{
		cm: cm,
	}
}

func TestUpdateField(t *testing.T) {
	ctx := context.Background()
	getUpdater := newGetUpdater(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetDataField("field1", "value1").
			SetDataField("field2", "value2").
			Build(),
	)
	err := UpdateField(ctx, getUpdater, nsName("namespace", "name"), "field1", "newValue")
	assert.NoError(t, err)
	val, _ := ReadKey(ctx, getUpdater, "field1", nsName("namespace", "name"))
	assert.Equal(t, "newValue", val)
	val2, _ := ReadKey(ctx, getUpdater, "field2", nsName("namespace", "name"))
	assert.Equal(t, "value2", val2)
}

func nsName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: namespace}
}

func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}
