package replica_set

import (
	"fmt"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/wait"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func Test(t *testing.T) {

}

func TestReplicaSetArbiter(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	type args struct {
		numberOfArbiters     int
		scaleArbitersTo      int
		numberOfMembers      int
		expectedErrorMessage string
		resourceName         string
	}
	tests := map[string]args{
		"Number of Arbiters must be less than number of nodes": {
			numberOfArbiters:     3,
			numberOfMembers:      3,
			expectedErrorMessage: fmt.Sprintf("error validating new Spec: number of arbiters specified (%v) is greater or equal than the number of members in the replicaset (%v). At least one member must not be an arbiter", 3, 3),
			resourceName:         "mdb0",
		},
		"Number of Arbiters must be greater than 0": {
			numberOfArbiters:     -1,
			numberOfMembers:      3,
			expectedErrorMessage: "error validating new Spec: number of arbiters must be greater or equal than 0",
			resourceName:         "mdb1",
		},
		"Scaling arbiters from 0 to 1": {
			numberOfArbiters: 0,
			scaleArbitersTo:  1,
			numberOfMembers:  2,
			resourceName:     "mdb2",
		},
		"Scaling Arbiters from 1 to 0": {
			numberOfArbiters: 1,
			scaleArbitersTo:  0,
			numberOfMembers:  3,
			resourceName:     "mdb3",
		},
		"Arbiters can be deployed in initial bootstrap": {
			numberOfArbiters: 1,
			scaleArbitersTo:  1,
			numberOfMembers:  2,
			resourceName:     "mdb4",
		},
	}
	for testName, _ := range tests {
		t.Run(testName, func(t *testing.T) {
			testConfig, _ := tests[testName]
			mdb, user := e2eutil.NewTestMongoDB(ctx, testConfig.resourceName, "")
			mdb.Spec.Arbiters = testConfig.numberOfArbiters
			mdb.Spec.Members = testConfig.numberOfMembers
			pwd, err := setup.GeneratePasswordForUser(ctx, user, "")
			if err != nil {
				t.Fatal(err)
			}
			t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
			if len(testConfig.expectedErrorMessage) > 0 {
				t.Run("Check status", mongodbtests.StatefulSetMessageIsReceived(&mdb, ctx, testConfig.expectedErrorMessage))
			} else {
				t.Run("Check that the stateful set becomes ready", mongodbtests.StatefulSetBecomesReady(&mdb, wait.Timeout(20*time.Minute)))
				t.Run("Check the number of arbiters", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(&mdb, testConfig.numberOfArbiters))

				if testConfig.numberOfArbiters != testConfig.scaleArbitersTo {
					t.Run(fmt.Sprintf("Scale Arbiters to %v", testConfig.scaleArbitersTo), mongodbtests.ScaleArbiters(&mdb, testConfig.scaleArbitersTo))
					t.Run("Arbiters Stateful Set Scaled Correctly", mongodbtests.ArbitersStatefulSetBecomesReady(&mdb))
				}

				t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
				t.Run("Test SRV Connectivity with generated connection string secret", func(t *testing.T) {
					tester, err := mongotester.FromResource(t, mdb)
					if err != nil {
						t.Fatal(err)
					}
					scramUser := mdb.GetScramUsers()[0]
					expectedCnxStr := fmt.Sprintf("mongodb+srv://%s-user:%s@%s-svc.%s.svc.cluster.local/admin?replicaSet=%s&ssl=false", mdb.Name, pwd, mdb.Name, mdb.Namespace, mdb.Name)
					cnxStrSrv := mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser)
					assert.Equal(t, expectedCnxStr, cnxStrSrv)
					tester.ConnectivitySucceeds(mongotester.WithURI(cnxStrSrv))
				})
			}
			t.Run("Delete MongoDB Resource", mongodbtests.DeleteMongoDBResource(&mdb, ctx))
		})
	}
}
