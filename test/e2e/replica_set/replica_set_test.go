package replica_set

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSet(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	scramUser := mdb.GetAuthUsers()[0]

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	sizeThresholdMB := automationconfig.SizeThresholdMB("0.0001")
	percent := automationconfig.PercentOfDiskspace("1")

	lcr := automationconfig.LogRotate{
		SizeThresholdMB:                 &sizeThresholdMB,
		TimeThresholdHrs:                1,
		NumUncompressed:                 10,
		NumTotal:                        10,
		PercentOfDiskspace:              &percent,
		IncludeAuditLogsWithMongoDBLogs: false,
	}

	systemLog := automationconfig.SystemLog{
		Destination: automationconfig.File,
		Path:        "/tmp/path",
		LogAppend:   false,
	}

	// LogRotate can only be configured if systemLog to file has been configured
	mdb.Spec.AgentConfiguration.LogRotate = &lcr
	mdb.Spec.AgentConfiguration.SystemLog = &systemLog

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("AutomationConfig has the correct logRotateConfig", mongodbtests.AutomationConfigHasLogRotationConfig(&mdb, lcr))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet((mdb.Name))))
	t.Run("Test Basic Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
	t.Run("Test SRV Connectivity with generated connection string secret",
		tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
}
