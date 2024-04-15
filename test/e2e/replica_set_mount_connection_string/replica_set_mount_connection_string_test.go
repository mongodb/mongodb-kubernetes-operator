package replica_set_mount_connection_string

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// createPythonTestPod creates a pod with a simple python app which connects to a MongoDB database
// using the connection string referenced within a given secret key.
func createPythonTestPod(idx int, namespace, secretName, secretKey string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-pod-%d", idx),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:       "python-app",
					Image:      "quay.io/mongodb/mongodb-kubernetes-operator-test-app:1.0.0",
					Command:    []string{"python", "main.py"},
					WorkingDir: "/app",
					Env: []corev1.EnvVar{
						{
							Name: "CONNECTION_STRING",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
									Key: secretKey,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestMountConnectionString(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
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

	t.Run("Application Pod can connect to MongoDB using the generated standard connection string.", func(t *testing.T) {
		testPod := createPythonTestPod(0, mdb.Namespace, fmt.Sprintf("%s-admin-%s", mdb.Name, user.Name), "connectionString.standard")
		err := e2eutil.TestClient.Create(ctx, &testPod, &e2eutil.CleanupOptions{
			TestContext: testCtx,
		})
		assert.NoError(t, err)
		assert.NoError(t, wait.ForPodPhase(ctx, t, time.Minute*5, testPod, corev1.PodSucceeded))
	})

	t.Run("Application Pod can connect to MongoDB using the generated secret SRV connection string", func(t *testing.T) {
		testPod := createPythonTestPod(1, mdb.Namespace, fmt.Sprintf("%s-admin-%s", mdb.Name, user.Name), "connectionString.standardSrv")
		err := e2eutil.TestClient.Create(ctx, &testPod, &e2eutil.CleanupOptions{
			TestContext: testCtx,
		})
		assert.NoError(t, err)
		assert.NoError(t, wait.ForPodPhase(ctx, t, time.Minute*5, testPod, corev1.PodSucceeded))
	})
}
