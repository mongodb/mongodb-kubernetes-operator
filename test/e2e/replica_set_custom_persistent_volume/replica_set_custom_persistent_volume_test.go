package replica_set_custom_persistent_volume

import (
	"context"
	"fmt"
	"os"
	"testing"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// getPersistentVolumeLocal returns a persistentVolume of type localPath and a "type" label.
func getPersistentVolumeLocal(name string, localPath string, label string) corev1.PersistentVolume {
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: e2eutil.OperatorNamespace,
			Labels:    map[string]string{"type": label},
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path: localPath,
				},
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			StorageClassName: "default",
			Capacity:         corev1.ResourceList{corev1.ResourceStorage: *resource.NewScaledQuantity(int64(8), resource.Giga)},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: "In",
									Values:   []string{"kind-control-plane"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// getVolumes returns two persistentVolumes for each of the `members` pod.
// one volume will be for the `data` claim and the other will be for the `logs` claim
func getVolumes(ctx *e2eutil.TestContext, volumeType string, members int) []corev1.PersistentVolume {
	volumes := make([]corev1.PersistentVolume, members)
	for i := 0; i < members; i++ {
		volumes[i] = getPersistentVolumeLocal(
			fmt.Sprintf("%s-volume-%d", volumeType, i),
			fmt.Sprintf("/opt/data/mongo-%s-%d", volumeType, i),
			volumeType,
		)
	}
	return volumes
}

func getPvc(pvcType string, mdb v1.MongoDBCommunity) corev1.PersistentVolumeClaim {
	name := ""
	if pvcType == "logs" {
		name = mdb.LogsVolumeName()
	} else {
		name = mdb.DataVolumeName()
	}
	defaultStorageClass := "default"
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"type": pvcType},
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{"storage": *resource.NewScaledQuantity(int64(8), resource.Giga)},
			},
			StorageClassName: &defaultStorageClass,
		},
	}
}

func TestReplicaSetCustomPersistentVolumes(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
		getPvc("data", mdb),
		getPvc("logs", mdb),
	}
	volumesToCreate := getVolumes(testCtx, "data", mdb.Spec.Members)
	volumesToCreate = append(volumesToCreate, getVolumes(testCtx, "logs", mdb.Spec.Members)...)

	for i := range volumesToCreate {
		err := e2eutil.TestClient.Create(ctx, &volumesToCreate[i], &e2eutil.CleanupOptions{TestContext: testCtx})
		assert.NoError(t, err)
	}
	scramUser := mdb.GetAuthUsers()[0]

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet((mdb.Name))))
	t.Run("Test Basic Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))
	t.Run("Test SRV Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser))))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	t.Run("Cluster has the expected persistent volumes", mongodbtests.HasExpectedPersistentVolumes(ctx, volumesToCreate))
}
