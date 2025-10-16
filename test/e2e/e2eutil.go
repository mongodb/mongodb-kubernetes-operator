package e2eutil

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const testDataDirEnv = "TEST_DATA_DIR"

// TestLabels should be applied to all resources created by tests.
func TestLabels() map[string]string {
	return map[string]string{
		"e2e-test": "true",
	}
}

// TestAnnotations create an annotations map
func TestAnnotations() map[string]string {
	return map[string]string{
		"e2e-test-annotated": "true",
	}
}

func TestDataDir() string {
	return envvar.GetEnvOrDefault(testDataDirEnv, "/workspace/testdata") // nolint:forbidigo
}

func TlsTestDataDir() string {
	return fmt.Sprintf("%s/tls", TestDataDir())
}

// UpdateMongoDBResource applies the provided function to the most recent version of the MongoDB resource
// and retries when there are conflicts
func UpdateMongoDBResource(ctx context.Context, original *mdbv1.MongoDBCommunity, updateFunc func(*mdbv1.MongoDBCommunity)) error {
	err := TestClient.Get(ctx, types.NamespacedName{Name: original.Name, Namespace: original.Namespace}, original)
	if err != nil {
		return err
	}

	updateFunc(original)

	return TestClient.Update(ctx, original)
}

func NewTestMongoDB(ctx *TestContext, name string, namespace string) (mdbv1.MongoDBCommunity, mdbv1.MongoDBUser) {
	mongodbNamespace := namespace
	if mongodbNamespace == "" {
		mongodbNamespace = OperatorNamespace
	}
	mdb := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mongodbNamespace,
			Labels:    TestLabels(),
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members:  3,
			Type:     "ReplicaSet",
			Version:  "8.0.0",
			Arbiters: 0,
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			Users: []mdbv1.MongoDBUser{
				{
					Name: fmt.Sprintf("%s-user", name),
					PasswordSecretRef: mdbv1.SecretKeyReference{
						Key:  fmt.Sprintf("%s-password", name),
						Name: fmt.Sprintf("%s-%s-password-secret", name, ctx.ExecutionId),
					},
					Roles: []mdbv1.Role{
						// roles on testing db for general connectivity
						{
							DB:   "testing",
							Name: "readWrite",
						},
						{
							DB:   "testing",
							Name: "clusterAdmin",
						},
						// admin roles for reading FCV
						{
							DB:   "admin",
							Name: "readWrite",
						},
						{
							DB:   "admin",
							Name: "clusterAdmin",
						},
						{
							DB:   "admin",
							Name: "userAdmin",
						},
					},
					ScramCredentialsSecretName: fmt.Sprintf("%s-my-scram", name),
				},
			},
			StatefulSetConfiguration: mdbv1.StatefulSetConfiguration{
				SpecWrapper: mdbv1.StatefulSetSpecWrapper{
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "mongod",
										Resources: corev1.ResourceRequirements{
											Limits: map[corev1.ResourceName]resource.Quantity{
												"cpu":    resource.MustParse("1.0"),
												"memory": resource.MustParse("200M"),
											},
											Requests: map[corev1.ResourceName]resource.Quantity{
												"cpu":    resource.MustParse("0.1"),
												"memory": resource.MustParse("200M"),
											},
										},
									},
									{
										Name: "mongodb-agent",
										Resources: corev1.ResourceRequirements{
											Limits: map[corev1.ResourceName]resource.Quantity{
												"cpu":    resource.MustParse("1.0"),
												"memory": resource.MustParse("200M"),
											},
											Requests: map[corev1.ResourceName]resource.Quantity{
												"cpu":    resource.MustParse("0.1"),
												"memory": resource.MustParse("200M"),
											},
										},
									},
								},
							},
						},
					}},
			},
		},
	}
	return mdb, mdb.Spec.Users[0]
}

func NewTestTLSConfig(optional bool) mdbv1.TLS {
	return mdbv1.TLS{
		Enabled:  true,
		Optional: optional,
		CertificateKeySecret: corev1.LocalObjectReference{
			Name: "tls-certificate",
		},
		CaCertificateSecret: &corev1.LocalObjectReference{
			Name: "tls-ca-key-pair",
		},
	}
}

func NewPrometheusConfig(ctx context.Context, namespace string) *mdbv1.Prometheus {
	sec := secret.Builder().
		SetName("prom-secret").
		SetNamespace(namespace).
		SetField("password", "prom-password").
		Build()
	err := TestClient.Create(ctx, &sec, &CleanupOptions{})
	if err != nil {
		if !apiErrors.IsAlreadyExists(err) {
			panic(fmt.Sprintf("Error trying to create secret: %s", err))
		}
	}

	return &mdbv1.Prometheus{
		Username: "prom-user",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "prom-secret",
		},
	}
}

func ensureObject(ctx *TestContext, obj k8sClient.Object) error {
	key := k8sClient.ObjectKeyFromObject(obj)
	obj.SetLabels(TestLabels())

	err := TestClient.Get(ctx.Ctx, key, obj)
	if err != nil {
		if !apiErrors.IsNotFound(err) {
			return err
		}
		err = TestClient.Create(ctx.Ctx, obj, &CleanupOptions{TestContext: ctx})
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("%s %s/%s already exists!\n", reflect.TypeOf(obj), key.Namespace, key.Name)
		err = TestClient.Update(ctx.Ctx, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// EnsureNamespace checks that the given namespace exists and creates it if not.
func EnsureNamespace(ctx *TestContext, namespace string) error {
	return ensureObject(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: TestLabels(),
		},
	})
}
