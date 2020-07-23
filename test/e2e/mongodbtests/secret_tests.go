package mongodbtests

import (
	"context"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SecretHasKeys(key client.ObjectKey, keys ...string) func(t *testing.T) {
	return func(t *testing.T) {
		credSecret := corev1.Secret{}
		if err := f.Global.Client.Get(context.TODO(), key, &credSecret); err != nil {
			t.Fatal(err)
		}
		assert.True(t, secret.HasAllKeys(credSecret, keys...))
	}
}
