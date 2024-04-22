package replica_set_x509

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/tlstests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetX509(t *testing.T) {
	ctx := context.Background()
	resourceName := "mdb-tls"
	helmArgs := []setup.HelmArg{
		{Name: "resource.tls.useX509", Value: "true"},
		{Name: "resource.tls.sampleX509User", Value: "true"},
	}
	testCtx, testConfig := setup.SetupWithTLS(ctx, t, resourceName, helmArgs...)
	defer testCtx.Teardown()

	mdb, _ := e2eutil.NewTestMongoDB(testCtx, resourceName, testConfig.Namespace)
	mdb.Spec.Security.Authentication.Modes = []v1.AuthMode{"X509"}
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	tester, err := FromX509Resource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Connection with certificates of wrong user", func(t *testing.T) {
		mdb.Spec.Users = []v1.MongoDBUser{
			getInvalidUser(),
		}
		users := mdb.GetAuthUsers()

		t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
		t.Run("Basic tests", mongodbtests.BasicFunctionalityX509(ctx, &mdb))
		t.Run("Agent certificate secrets configured", mongodbtests.AgentX509SecretsExists(ctx, &mdb))

		cert, root, dir := createCerts(ctx, t, &mdb)
		defer os.RemoveAll(dir)

		t.Run("Connectivity Fails without certs", tester.ConnectivityFails(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0])), WithTls(ctx, mdb)))
		t.Run("Connectivity Fails with invalid certs", tester.ConnectivityFails(WithURI(fmt.Sprintf("%s&tlsCAFile=%s&tlsCertificateKeyFile=%s", mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0]), root, cert))))
	})

	t.Run("Connection with valid certificate", func(t *testing.T) {
		t.Run("Update MongoDB Resource", func(t *testing.T) {
			err := e2eutil.UpdateMongoDBResource(ctx, &mdb, func(m *v1.MongoDBCommunity) {
				m.Spec.Users = []v1.MongoDBUser{getValidUser()}
			})
			assert.NoError(t, err)
		})

		cert, root, dir := createCerts(ctx, t, &mdb)
		defer os.RemoveAll(dir)

		users := mdb.GetAuthUsers()

		t.Run("Basic tests", mongodbtests.BasicFunctionalityX509(ctx, &mdb))
		t.Run("Agent certificate secrets configured", mongodbtests.AgentX509SecretsExists(ctx, &mdb))
		t.Run("Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(fmt.Sprintf("%s&tlsCAFile=%s&tlsCertificateKeyFile=%s", mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0]), root, cert))))
	})

	t.Run("Rotate agent certificate", func(t *testing.T) {
		agentCert, err := GetAgentCert(ctx, mdb)
		if err != nil {
			t.Fatal(err)
		}
		initialCertSerialNumber := agentCert.SerialNumber

		initialAgentPem := &corev1.Secret{}
		err = e2eutil.TestClient.Get(ctx, mdb.AgentCertificatePemSecretNamespacedName(), initialAgentPem)
		assert.NoError(t, err)

		cert, root, dir := createCerts(ctx, t, &mdb)
		defer os.RemoveAll(dir)

		users := mdb.GetAuthUsers()

		t.Run("Update certificate secret", tlstests.RotateAgentCertificate(ctx, &mdb))
		t.Run("Wait for MongoDB to reach Running Phase after rotating agent cert", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))

		agentCert, err = GetAgentCert(ctx, mdb)
		if err != nil {
			t.Fatal(err)
		}
		finalCertSerialNumber := agentCert.SerialNumber

		assert.NotEqual(t, finalCertSerialNumber, initialCertSerialNumber)

		finalAgentPem := &corev1.Secret{}
		err = e2eutil.TestClient.Get(ctx, mdb.AgentCertificatePemSecretNamespacedName(), finalAgentPem)
		assert.NoError(t, err)

		assert.NotEqual(t, finalAgentPem.Data, initialAgentPem.Data)

		t.Run("Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(fmt.Sprintf("%s&tlsCAFile=%s&tlsCertificateKeyFile=%s", mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0]), root, cert))))
	})

	t.Run("Transition to also allow SCRAM", func(t *testing.T) {
		t.Run("Update MongoDB Resource", func(t *testing.T) {
			err := e2eutil.UpdateMongoDBResource(ctx, &mdb, func(m *v1.MongoDBCommunity) {
				m.Spec.Security.Authentication.Modes = []v1.AuthMode{"X509", "SCRAM"}
				m.Spec.Security.Authentication.AgentMode = "X509"
			})
			assert.NoError(t, err)
		})

		cert, root, dir := createCerts(ctx, t, &mdb)
		defer os.RemoveAll(dir)

		users := mdb.GetAuthUsers()

		t.Run("Basic tests", mongodbtests.BasicFunctionalityX509(ctx, &mdb))
		t.Run("Agent certificate secrets configured", mongodbtests.AgentX509SecretsExists(ctx, &mdb))
		t.Run("Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(fmt.Sprintf("%s&tlsCAFile=%s&tlsCertificateKeyFile=%s", mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0]), root, cert))))
	})

	t.Run("Transition to SCRAM agent", func(t *testing.T) {
		t.Run("Update MongoDB Resource", func(t *testing.T) {
			err := e2eutil.UpdateMongoDBResource(ctx, &mdb, func(m *v1.MongoDBCommunity) {
				m.Spec.Security.Authentication.AgentMode = "SCRAM"
			})
			assert.NoError(t, err)
		})

		cert, root, dir := createCerts(ctx, t, &mdb)
		defer os.RemoveAll(dir)

		users := mdb.GetAuthUsers()

		t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
		t.Run("Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(fmt.Sprintf("%s&tlsCAFile=%s&tlsCertificateKeyFile=%s", mongodbtests.GetConnectionStringForUser(ctx, mdb, users[0]), root, cert))))
	})

}

func getValidUser() v1.MongoDBUser {
	return v1.MongoDBUser{
		Name: "CN=my-x509-user,OU=organizationalunit,O=organization",
		DB:   constants.ExternalDB,
		Roles: []v1.Role{
			{
				DB:   "admin",
				Name: "readWriteAnyDatabase",
			},
			{
				DB:   "admin",
				Name: "clusterAdmin",
			},
			{
				DB:   "admin",
				Name: "userAdminAnyDatabase",
			},
		},
	}
}

func getInvalidUser() v1.MongoDBUser {
	return v1.MongoDBUser{
		Name: "CN=my-invalid-x509-user,OU=organizationalunit,O=organization",
		DB:   constants.ExternalDB,
		Roles: []v1.Role{
			{
				DB:   "admin",
				Name: "readWriteAnyDatabase",
			},
			{
				DB:   "admin",
				Name: "clusterAdmin",
			},
			{
				DB:   "admin",
				Name: "userAdminAnyDatabase",
			},
		},
	}
}

func createCerts(ctx context.Context, t *testing.T, mdb *v1.MongoDBCommunity) (string, string, string) {
	dir, _ := os.MkdirTemp("", "certdir")

	t.Logf("Creating client certificate pem file")
	cert, _ := os.CreateTemp(dir, "pem")
	clientCertSecret := corev1.Secret{}
	err := e2eutil.TestClient.Get(ctx, types.NamespacedName{
		Namespace: mdb.Namespace,
		Name:      "my-x509-user-cert",
	}, &clientCertSecret)
	assert.NoError(t, err)

	_, err = cert.Write(append(clientCertSecret.Data["tls.crt"], clientCertSecret.Data["tls.key"]...))
	assert.NoError(t, err)
	t.Logf("Created pem file: %s", cert.Name())

	t.Logf("Creating root ca file")
	root, _ := os.CreateTemp(dir, "root")
	_, err = root.Write(clientCertSecret.Data["ca.crt"])
	assert.NoError(t, err)
	t.Logf("Created root ca file: %s", root.Name())

	return cert.Name(), root.Name(), dir
}
