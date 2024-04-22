package headless

import (
	"context"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/cmd/readiness/testdata"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPerformCheckHeadlessMode(t *testing.T) {
	ctx := context.Background()
	c := testConfig()

	c.ClientSet = fake.NewSimpleClientset(testdata.TestPod(c.Namespace, c.Hostname), testdata.TestSecret(c.Namespace, c.AutomationConfigSecretName, 11))
	status := health.Status{
		MmsStatus: map[string]health.MmsDirectorStatus{c.Hostname: {
			LastGoalStateClusterConfigVersion: 10,
		}},
	}

	achieved, err := PerformCheckHeadlessMode(ctx, status, c)

	require.NoError(t, err)
	assert.False(t, achieved)

	thePod, _ := c.ClientSet.CoreV1().Pods(c.Namespace).Get(ctx, c.Hostname, metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "10"}, thePod.Annotations)
}

func testConfig() config.Config {
	return config.Config{
		Namespace:                  "test-ns",
		AutomationConfigSecretName: "test-mongodb-automation-config",
		Hostname:                   "test-mongodb-0",
	}
}
