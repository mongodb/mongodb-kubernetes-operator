package replica_set_custom_persistent_volume

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

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

func getVolumes(ctx *e2eutil.Context, volumeType string, members int) []corev1.PersistentVolume {
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

func TestReplicaSetCustomPersistentVolumes(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	volumesToCreate := getVolumes(ctx, "data", mdb.Spec.Members)
	volumesToCreate = append(volumesToCreate, getVolumes(ctx, "logs", mdb.Spec.Members)...)

	for i := range volumesToCreate {
		err := e2eutil.TestClient.Create(context.TODO(), &volumesToCreate[i], &e2eutil.CleanupOptions{TestContext: ctx})
		assert.NoError(t, err)
	}
	scramUser := mdb.GetScramUsers()[0]

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI()), WithoutTls(), WithReplicaSet((mdb.Name))))
	t.Run("Test Basic Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
	t.Run("Test SRV Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Cluster has the expected persistent volumes", mongodbtests.HasExpectedPersistentVolumes(volumesToCreate))
}
