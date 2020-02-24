package mongodb

import (
	"context"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/mock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestKubernetesResources_AreCreated(t *testing.T) {
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-rs",
			Namespace: "my-ns",
		},
		Spec:   mdbv1.MongoDBSpec{},
		Status: mdbv1.MongoDBStatus{},
	}

	mgr := mock.NewManager(&mdb)
	r := newReconciler(mgr)

	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	cm := corev1.ConfigMap{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &cm)
	assert.NoError(t, err)
	assert.Equal(t, mdb.Namespace, cm.Namespace)
	assert.Equal(t, mdb.ConfigMapName(), cm.Name)
	assert.Contains(t, cm.Data, automationConfigKey)
	assert.NotEmpty(t, cm.Data[automationConfigKey])
}
