package statefulset_arbitrary_config_update

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	corev1 "k8s.io/api/core/v1"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestStatefulSetArbitraryConfig(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}
	mdb, user := e2eutil.NewTestMongoDB("mdb0", "")

	_, err := setup.GeneratePasswordForUser(user, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	mdb.Spec.StatefulSetConfiguration.Template.Spec.Containers = []corev1.Container{
		{Name: "mongodb-agent", ReadinessProbe: &corev1.Probe{TimeoutSeconds: 100}},
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Container has been merged by name", mongodbtests.StatefulSetContainerConditionIsTrue(&mdb, "mongodb-agent", func(container corev1.Container) bool {
		return container.ReadinessProbe.TimeoutSeconds == 100
	}))
}
