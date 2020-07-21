package statefulset_arbitrary_config

import (
	"context"
	"testing"

	"github.com/golangplus/testing/assert"
	v1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestStatefulSetArbitraryConfig(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}
	mdb, user := e2eutil.NewTestMongoDB("mdb0")

	_, err := setup.GeneratePasswordForUser(user, ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", mongodbtests.Connectivity(&mdb))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

	// Get the original containers
	originalSts := &appsv1.StatefulSet{}
	err = f.Global.Client.Get(context.TODO(), mdb.NamespacedName(), originalSts)
	assert.NoError(t, err)

	overrideSpec := v1.StatefulSetConfiguration{}
	overrideSpec.Spec.Template.Spec.Containers = []corev1.Container{
		{Name: "mongodb-agent", ReadinessProbe: &corev1.Probe{TimeoutSeconds: 100}}}

	e2eutil.UpdateMongoDBResource(&mdb, func(mdb *v1.MongoDB) { mdb.Spec.StatefulSetConfiguration = overrideSpec })

	t.Run("Container has been merged by name", mongodbtests.StatefulSetContainerConditionIsTrue(mdb, "mongodb-agent", func(container corev1.Container) { return container.ReadinessProbe.TimeoutSeconds == 100 }))

}
