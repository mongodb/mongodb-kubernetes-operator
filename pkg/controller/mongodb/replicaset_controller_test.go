package mongodb

import (
	"context"
	"os"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	os.Setenv("AGENT_IMAGE", "agent-image")
}

func newTestReplicaSet() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-rs",
			Namespace: "my-ns",
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
		},
	}
}

func TestKubernetesResources_AreCreated(t *testing.T) {
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr)

	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	cm := corev1.ConfigMap{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &cm)
	assert.NoError(t, err)
	assert.Equal(t, mdb.Namespace, cm.Namespace)
	assert.Equal(t, mdb.ConfigMapName(), cm.Name)
	assert.Contains(t, cm.Data, AutomationConfigKey)
	assert.NotEmpty(t, cm.Data[AutomationConfigKey])
}

func TestStatefulSet_IsCorrectlyConfigured(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr)
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, agentName, agentContainer.Name)
	assert.Equal(t, os.Getenv(agentImageEnvVariable), agentContainer.Image)

	assert.Equal(t, resourcerequirements.Defaults(), agentContainer.Resources)
}
