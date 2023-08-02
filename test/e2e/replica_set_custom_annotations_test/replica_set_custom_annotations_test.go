package replica_set_custom_annotations_test

import (
	"fmt"
	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetCustomAnnotations(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Labels:      e2eutil.TestLabels(),
		Annotations: e2eutil.TestAnnotations(),
	}
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "data-volume",
				Labels:      e2eutil.TestLabels(),
				Annotations: e2eutil.TestAnnotations(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "logs-volume",
				Labels:      e2eutil.TestLabels(),
				Annotations: e2eutil.TestAnnotations(),
			},
		},
	}
	mdb.Spec.StatefulSetConfiguration.MetadataWrapper = v1.StatefulSetMetadataWrapper{
		Labels:      e2eutil.TestLabels(),
		Annotations: e2eutil.TestAnnotations(),
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
	t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet((mdb.Name))))
	t.Run("Test Basic Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
	t.Run("Test SRV Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Cluster has the expected labels and annotations", mongodbtests.HasExpectedMetadata(&mdb, e2eutil.TestLabels(), e2eutil.TestAnnotations()))
}
