package statefulset_arbitrary_config

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/stretchr/testify/assert"
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
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "mongodb-rs")

	_, err := setup.GeneratePasswordForUser(ctx, user, "mongodb-rs")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test basic connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

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

	overrideSpec := mdb.Spec.StatefulSetConfiguration
	overrideSpec.SpecWrapper.Spec.Template.Spec.Containers[1].ReadinessProbe = &corev1.Probe{TimeoutSeconds: 100}
	overrideSpec.SpecWrapper.Spec.Template.Spec.Tolerations = overrideTolerations

	err = e2eutil.UpdateMongoDBResource(&mdb, func(mdb *mdbv1.MongoDBCommunity) { mdb.Spec.StatefulSetConfiguration = overrideSpec })

	assert.NoError(t, err)

	t.Run("Basic tests after update", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test basic connectivity after update", tester.ConnectivitySucceeds())
	t.Run("Container has been merged by name", mongodbtests.StatefulSetContainerConditionIsTrue(&mdb, "mongodb-agent", func(container corev1.Container) bool {
		return container.ReadinessProbe.TimeoutSeconds == 100
	}))
	t.Run("Tolerations have been added correctly", mongodbtests.StatefulSetConditionIsTrue(&mdb, func(sts appsv1.StatefulSet) bool {
		tolerationsFound := true
		for _, overrideToleration := range overrideTolerations {
			tolerationFound := false
			for _, specToleration := range sts.Spec.Template.Spec.Tolerations {
				if reflect.DeepEqual(overrideToleration, specToleration) {
					tolerationFound = true
					break
				}
			}
			if !tolerationFound {
				tolerationsFound = false
			}
		}
		return tolerationsFound
	}))
}
