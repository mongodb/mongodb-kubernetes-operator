package setup

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	skipCleanup = "SKIP_CLEANUP"
)

func InitTest(t *testing.T) (*f.Context, bool) {
	ctx := f.NewContext(t)

	if err := registerTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	skip := os.Getenv(skipCleanup)

	return ctx, skip != "True"
}

func registerTypesWithFramework(newTypes ...runtime.Object) error {
	for _, newType := range newTypes {
		if err := f.AddToFrameworkScheme(apis.AddToScheme, newType); err != nil {
			return fmt.Errorf("failed to add custom resource type %s to framework scheme: %v", newType.GetObjectKind(), err)
		}
	}
	return nil
}
