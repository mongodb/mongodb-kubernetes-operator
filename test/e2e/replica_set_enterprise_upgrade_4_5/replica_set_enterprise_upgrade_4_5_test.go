package replica_set

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/replica_set_enterprise_upgrade"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
)

var (
	versionsForUpgrades = []string{"4.4.19", "5.0.15"}
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSet(t *testing.T) {
	ctx := context.Background()
	replica_set_enterprise_upgrade.DeployEnterpriseAndUpgradeTest(ctx, t, versionsForUpgrades)
}
