package mongodb

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

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
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Version: "4.2.2",
		},
	}
}

func mockManifestProvider(version string) func() (automationconfig.VersionManifest, error) {
	return func() (automationconfig.VersionManifest, error) {
		return automationconfig.VersionManifest{
			Updated: 0,
			Versions: []automationconfig.MongoDbVersionConfig{
				{
					Name: version,
					Builds: []automationconfig.BuildConfig{{
						Platform:     "platform",
						Url:          "url",
						GitVersion:   "gitVersion",
						Architecture: "arch",
						Flavor:       "flavor",
						MinOsVersion: "0",
						MaxOsVersion: "10",
						Modules:      []string{},
					}},
				}},
		}, nil
	}
}

func TestKubernetesResources_AreCreated(t *testing.T) {
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))

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
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, agentName, agentContainer.Name)
	assert.Equal(t, os.Getenv(agentImageEnv), agentContainer.Image)
	expectedProbe := defaultReadinessProbe()
	assert.True(t, reflect.DeepEqual(&expectedProbe, agentContainer.ReadinessProbe))

	mongodbContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, mongodbName, mongodbContainer.Name)
	assert.Equal(t, "mongo:4.2.2", mongodbContainer.Image)

	assert.Equal(t, resourcerequirements.Defaults(), agentContainer.Resources)
}

func TestChangingVersion_ResultsInRollingUpdateStrategyType(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	mgrClient := mgr.GetClient()
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	// fetch updated resource after first reconciliation
	_ = mgrClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &mdb)

	sts := appsv1.StatefulSet{}
	err = mgrClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)
	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)

	mdbRef := &mdb
	mdbRef.Spec.Version = "4.2.3"

	_ = mgrClient.Update(context.TODO(), &mdb)

	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgrClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type,
		"The StatefulSet should have be re-configured to use RollingUpdates after it reached the ready state")
}

//
//func TestBuildStatefulSet_ConfiguresUpdateStrategyCorrectly(t *testing.T) {
//	t.Run("On No Version Change, Same Version", func(t *testing.T) {
//		mdb := newTestReplicaSet()
//		mdb.Spec.Version = "4.0.0"
//		mdb.Annotations[mdbv1.ReachedVersionAnnotationKey] = "4.0.0"
//		sts, err := buildStatefulSet(mdb)
//		assert.NoError(t, err)
//		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
//	})
//	t.Run("On No Version Change, First Version", func(t *testing.T) {
//		mdb := newTestReplicaSet()
//		mdb.Spec.Version = "4.0.0"
//		delete(mdb.Annotations, mdbv1.ReachedVersionAnnotationKey)
//		sts, err := buildStatefulSet(mdb)
//		assert.NoError(t, err)
//		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
//	})
//	t.Run("On Version Change", func(t *testing.T) {
//		mdb := newTestReplicaSet()
//		mdb.Spec.Version = "4.0.0"
//		mdb.Annotations[mdbv1.ReachedVersionAnnotationKey] = "4.2.0"
//		sts, err := buildStatefulSet(mdb)
//		assert.NoError(t, err)
//		assert.Equal(t, appsv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
//	})
//}
