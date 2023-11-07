package replica_set

import (
	"fmt"
	"os"
	"testing"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/replica_set_enterprise_upgrade"
)

var (
	versionsForUpgrades = []string{"6.0.5", "7.0.2"}
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSet(t *testing.T) {
	replica_set_enterprise_upgrade.DeployEnterpriseAndUpgradeTest(t, versionsForUpgrades)
}
