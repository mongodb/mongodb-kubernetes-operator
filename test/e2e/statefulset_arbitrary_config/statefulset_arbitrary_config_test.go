package statefulset_arbitrary_config_update

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	appsv1 "k8s.io/api/apps/v1"
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
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	overrideTolerations := []corev1.Toleration{
		{
			Key:      "key1",
			Value:    "value1",
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "key2",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectPreferNoSchedule,
		},
	}

	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.Template.Spec.Containers[1].ReadinessProbe = &corev1.Probe{TimeoutSeconds: 100}
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.Template.Spec.Tolerations = overrideTolerations

	customServiceName := "database"
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.ServiceName = customServiceName

	tester, err := mongotester.FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Test setting Service Name", mongodbtests.ServiceWithNameExists(ctx, customServiceName, mdb.Namespace))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	t.Run("Container has been merged by name", mongodbtests.StatefulSetContainerConditionIsTrue(ctx, &mdb, "mongodb-agent", func(container corev1.Container) bool {
		return container.ReadinessProbe.TimeoutSeconds == 100
	}))
	t.Run("Tolerations have been added correctly", mongodbtests.StatefulSetConditionIsTrue(ctx, &mdb, func(sts appsv1.StatefulSet) bool {
		return reflect.DeepEqual(overrideTolerations, sts.Spec.Template.Spec.Tolerations)
	}))
}
