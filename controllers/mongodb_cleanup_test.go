package controllers

import (
	"context"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	kubeClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReplicaSetReconcilerCleanupScramSecrets(t *testing.T) {
	lastApplied := newScramReplicaSet(mdbv1.MongoDBUser{
		Name: "testUser",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "password-secret-name",
		},
		ScramCredentialsSecretName: "scram-credentials",
	})

	t.Run("no change same resource", func(t *testing.T) {
		actual := getScramSecretsToDelete(lastApplied.Spec, lastApplied.Spec)

		assert.Equal(t, []string(nil), actual)
	})

	t.Run("new user new secret", func(t *testing.T) {
		current := newScramReplicaSet(
			mdbv1.MongoDBUser{
				Name: "testUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ScramCredentialsSecretName: "scram-credentials",
			},
			mdbv1.MongoDBUser{
				Name: "newUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ScramCredentialsSecretName: "scram-credentials-2",
			},
		)

		actual := getScramSecretsToDelete(current.Spec, lastApplied.Spec)

		assert.Equal(t, []string(nil), actual)
	})

	t.Run("old user new secret", func(t *testing.T) {
		current := newScramReplicaSet(mdbv1.MongoDBUser{
			Name: "testUser",
			PasswordSecretRef: mdbv1.SecretKeyReference{
				Name: "password-secret-name",
			},
			ScramCredentialsSecretName: "scram-credentials-2",
		})

		expected := []string{"scram-credentials-scram-credentials"}
		actual := getScramSecretsToDelete(current.Spec, lastApplied.Spec)

		assert.Equal(t, expected, actual)
	})

	t.Run("removed one user and changed secret of the other", func(t *testing.T) {
		lastApplied = newScramReplicaSet(
			mdbv1.MongoDBUser{
				Name: "testUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ScramCredentialsSecretName: "scram-credentials",
			},
			mdbv1.MongoDBUser{
				Name: "anotherUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ScramCredentialsSecretName: "another-scram-credentials",
			},
		)

		current := newScramReplicaSet(mdbv1.MongoDBUser{
			Name: "testUser",
			PasswordSecretRef: mdbv1.SecretKeyReference{
				Name: "password-secret-name",
			},
			ScramCredentialsSecretName: "scram-credentials-2",
		})

		expected := []string{"scram-credentials-scram-credentials", "another-scram-credentials-scram-credentials"}
		actual := getScramSecretsToDelete(current.Spec, lastApplied.Spec)

		assert.Equal(t, expected, actual)
	})

}
func TestReplicaSetReconcilerCleanupPemSecret(t *testing.T) {
	ctx := context.Background()
	lastAppliedSpec := mdbv1.MongoDBCommunitySpec{
		Security: mdbv1.Security{
			Authentication: mdbv1.Authentication{
				Modes: []mdbv1.AuthMode{"X509"},
			},
		},
	}
	mdb := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.2.2",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
				TLS: mdbv1.TLS{
					Enabled: true,
					CaConfigMap: &corev1.LocalObjectReference{
						Name: "caConfigMap",
					},
					CaCertificateSecret: &corev1.LocalObjectReference{
						Name: "certificateKeySecret",
					},
					CertificateKeySecret: corev1.LocalObjectReference{
						Name: "certificateKeySecret",
					},
				},
			},
		},
	}

	mgr := kubeClient.NewManager(ctx, &mdb)

	client := kubeClient.NewClient(mgr.GetClient())
	err := createAgentCertPemSecret(ctx, client, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

	secret, err := r.client.GetSecret(ctx, mdb.AgentCertificatePemSecretNamespacedName())
	assert.NoError(t, err)
	assert.Equal(t, "CERT", string(secret.Data["tls.crt"]))
	assert.Equal(t, "KEY", string(secret.Data["tls.key"]))

	r.cleanupPemSecret(ctx, mdb.Spec, lastAppliedSpec, "my-ns")

	_, err = r.client.GetSecret(ctx, mdb.AgentCertificatePemSecretNamespacedName())
	assert.Error(t, err)
}

func TestReplicaSetReconcilerCleanupConnectionStringSecrets(t *testing.T) {
	lastApplied := newScramReplicaSet(mdbv1.MongoDBUser{
		Name: "testUser",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "password-secret-name",
		},
		ConnectionStringSecretName: "connection-string-secret",
	})

	t.Run("no change same resource", func(t *testing.T) {
		actual := getConnectionStringSecretsToDelete(lastApplied.Spec, lastApplied.Spec, "my-rs")

		assert.Equal(t, []string(nil), actual)
	})

	t.Run("new user does not require existing user cleanup", func(t *testing.T) {
		current := newScramReplicaSet(
			mdbv1.MongoDBUser{
				Name: "testUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ConnectionStringSecretName: "connection-string-secret",
			},
			mdbv1.MongoDBUser{
				Name: "newUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ConnectionStringSecretName: "connection-string-secret-2",
			},
		)

		actual := getConnectionStringSecretsToDelete(current.Spec, lastApplied.Spec, "my-rs")

		assert.Equal(t, []string(nil), actual)
	})

	t.Run("old user new secret", func(t *testing.T) {
		current := newScramReplicaSet(mdbv1.MongoDBUser{
			Name: "testUser",
			PasswordSecretRef: mdbv1.SecretKeyReference{
				Name: "password-secret-name",
			},
			ConnectionStringSecretName: "connection-string-secret-2",
		})

		expected := []string{"connection-string-secret"}
		actual := getConnectionStringSecretsToDelete(current.Spec, lastApplied.Spec, "my-rs")

		assert.Equal(t, expected, actual)
	})

	t.Run("removed one user and changed secret of the other", func(t *testing.T) {
		lastApplied = newScramReplicaSet(
			mdbv1.MongoDBUser{
				Name: "testUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ConnectionStringSecretName: "connection-string-secret",
			},
			mdbv1.MongoDBUser{
				Name: "anotherUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ConnectionStringSecretName: "connection-string-secret-2",
			},
		)

		current := newScramReplicaSet(mdbv1.MongoDBUser{
			Name: "testUser",
			PasswordSecretRef: mdbv1.SecretKeyReference{
				Name: "password-secret-name",
			},
			ConnectionStringSecretName: "connection-string-secret-1",
		})

		expected := []string{"connection-string-secret", "connection-string-secret-2"}
		actual := getConnectionStringSecretsToDelete(current.Spec, lastApplied.Spec, "my-rs")

		assert.Equal(t, expected, actual)
	})

}
