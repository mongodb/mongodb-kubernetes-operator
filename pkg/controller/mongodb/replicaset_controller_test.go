package mongodb

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	os.Setenv("AGENT_IMAGE", "agent-image")
}

func newTestReplicaSet() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Version: "4.2.2",
		},
	}
}

func newScramReplicaSet() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Version: "4.2.2",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Enabled: true,
					Modes:   []mdbv1.AuthMode{"SCRAM"},
				},
			},
		},
	}
}

func newTestReplicaSetWithTLS() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Version: "4.2.2",
			Security: mdbv1.Security{
				TLS: mdbv1.TLS{
					Enabled: true,
					CaConfigMap: mdbv1.LocalObjectReference{
						Name: "caConfigMap",
					},
					CertificateKeySecret: mdbv1.LocalObjectReference{
						Name: "certificateKeySecret",
					},
				},
			},
		},
	}
}

func mockManifestProvider(version string) func() (automationconfig.VersionManifest, error) {
	return func() (automationconfig.VersionManifest, error) {
		return automationconfig.VersionManifest{
			Updated: 0,
			Versions: []automationconfig.MongoDbVersionConfig{
				{
					Name: version,
					Builds: []automationconfig.BuildConfig{{
						Platform:     "platform",
						Url:          "url",
						GitVersion:   "gitVersion",
						Architecture: "arch",
						Flavor:       "flavor",
						MinOsVersion: "0",
						MaxOsVersion: "10",
						Modules:      []string{},
					}},
				}},
		}, nil
	}
}

func TestKubernetesResources_AreCreated(t *testing.T) {
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))

	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	cm := corev1.ConfigMap{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &cm)
	assert.NoError(t, err)
	assert.Equal(t, mdb.Namespace, cm.Namespace)
	assert.Equal(t, mdb.ConfigMapName(), cm.Name)
	assert.Contains(t, cm.Data, AutomationConfigKey)
	assert.NotEmpty(t, cm.Data[AutomationConfigKey])
}

func TestStatefulSet_IsCorrectlyConfigured(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, agentName, agentContainer.Name)
	assert.Equal(t, os.Getenv(agentImageEnv), agentContainer.Image)
	expectedProbe := probes.New(defaultReadiness())
	assert.True(t, reflect.DeepEqual(&expectedProbe, agentContainer.ReadinessProbe))

	mongodbContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, mongodbName, mongodbContainer.Name)
	assert.Equal(t, "mongo:4.2.2", mongodbContainer.Image)

	assert.Equal(t, resourcerequirements.Defaults(), agentContainer.Resources)
}

func TestChangingVersion_ResultsInRollingUpdateStrategyType(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	mgrClient := mgr.GetClient()
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)

	// fetch updated resource after first reconciliation
	_ = mgrClient.Get(context.TODO(), mdb.NamespacedName(), &mdb)

	sts := appsv1.StatefulSet{}
	err = mgrClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)
	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)

	mdbRef := &mdb
	mdbRef.Spec.Version = "4.2.3"

	_ = mgrClient.Update(context.TODO(), &mdb)

	// agents start the upgrade, they are not all ready
	sts.Status.UpdatedReplicas = 1
	sts.Status.ReadyReplicas = 2
	err = mgrClient.Update(context.TODO(), &sts)
	assert.NoError(t, err)
	_ = mgrClient.Get(context.TODO(), mdb.NamespacedName(), &sts)

	// the request is requeued as the agents are still doing the upgrade
	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assert.NoError(t, err)
	assert.Equal(t, res.RequeueAfter, time.Second*10)

	_ = mgrClient.Get(context.TODO(), mdb.NamespacedName(), &sts)
	assert.Equal(t, appsv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	// upgrade is now complete
	sts.Status.UpdatedReplicas = 3
	sts.Status.ReadyReplicas = 3
	err = mgrClient.Update(context.TODO(), &sts)
	assert.NoError(t, err)

	// reconcilliation is successful
	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgrClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type,
		"The StatefulSet should have be re-configured to use RollingUpdates after it reached the ready state")
}

func TestBuildStatefulSet_ConfiguresUpdateStrategyCorrectly(t *testing.T) {
	t.Run("On No Version Change, Same Version", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		mdb.Annotations[lastVersionAnnotationKey] = "4.0.0"
		sts, err := buildStatefulSet(mdb)
		assert.NoError(t, err)
		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
	t.Run("On No Version Change, First Version", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		delete(mdb.Annotations, lastVersionAnnotationKey)
		sts, err := buildStatefulSet(mdb)
		assert.NoError(t, err)
		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
	t.Run("On Version Change", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		mdb.Annotations[lastVersionAnnotationKey] = "4.2.0"
		sts, err := buildStatefulSet(mdb)
		assert.NoError(t, err)
		assert.Equal(t, appsv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
}

func TestService_isCorrectlyCreatedAndUpdated(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 27017})

	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func TestAutomationConfig_versionIsBumpedOnChange(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := getCurrentAutomationConfig(client.NewClient(mgr.GetClient()), mdb)
	assert.NoError(t, err)
	assert.Equal(t, 1, currentAc.Version)

	mdb.Spec.Members++
	makeStatefulSetReady(mgr.GetClient(), mdb)

	_ = mgr.GetClient().Update(context.TODO(), &mdb)
	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = getCurrentAutomationConfig(client.NewClient(mgr.GetClient()), mdb)
	assert.NoError(t, err)
	assert.Equal(t, 2, currentAc.Version)
}

func TestAutomationConfig_versionIsNotBumpedWithNoChanges(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := getCurrentAutomationConfig(client.NewClient(mgr.GetClient()), mdb)
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)

	res, err = r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = getCurrentAutomationConfig(client.NewClient(mgr.GetClient()), mdb)
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)
}

func TestExistingPasswordAndKeyfile_AreUsedWhenTheSecretExists(t *testing.T) {
	mdb := newScramReplicaSet()
	mgr := client.NewManager(&mdb)

	c := mgr.Client

	scramNsName := mdb.ScramCredentialsNamespacedName()
	_ = secret.CreateOrUpdate(c,
		secret.Builder().
			SetName(scramNsName.Name).
			SetNamespace(scramNsName.Namespace).
			SetField(scram.AgentPasswordKey, "my-pass").
			SetField(scram.AgentKeyfileKey, "my-keyfile").
			Build(),
	)

	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := getCurrentAutomationConfig(c, mdb)
	assert.NoError(t, err)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.False(t, currentAc.Auth.Disabled)

	assert.Equal(t, "my-keyfile", currentAc.Auth.Key)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.Equal(t, "my-pass", currentAc.Auth.AutoPwd)

}

func TestScramIsConfigured(t *testing.T) {
	mdb := newScramReplicaSet()
	mgr := client.NewManager(&mdb)
	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := getCurrentAutomationConfig(client.NewClient(mgr.GetClient()), mdb)
	t.Run("Automation Config is configured with SCRAM", func(t *testing.T) {
		assert.NoError(t, err)
		assert.NotEmpty(t, currentAc.Auth.Key)
		assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
		assert.NotEmpty(t, currentAc.Auth.AutoPwd)
		assert.False(t, currentAc.Auth.Disabled)
	})
	t.Run("Secret with credentials was created", func(t *testing.T) {
		secretNsName := mdb.ScramCredentialsNamespacedName()
		s, err := mgr.Client.GetSecret(secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[scram.AgentKeyfileKey], []byte(currentAc.Auth.Key))
		assert.Equal(t, s.Data[scram.AgentPasswordKey], []byte(currentAc.Auth.AutoPwd))
	})
}

func TestStatefulSet_IsCorrectlyConfiguredWithTLS(t *testing.T) {
	mdb := newTestReplicaSetWithTLS()
	mgr := client.NewManager(&mdb)

	s := secret.Builder().
		SetName(mdb.Spec.Security.TLS.CertificateKeySecret.Name).
		SetNamespace(mdb.Namespace).
		SetField("tls.crt", "CERT").
		SetField("tls.key", "KEY").
		Build()

	err := mgr.GetClient().Create(context.TODO(), &s)
	assert.NoError(t, err)

	configMap := configmap.Builder().
		SetName(mdb.Spec.Security.TLS.CaConfigMap.Name).
		SetNamespace(mdb.Namespace).
		SetField("ca.crt", "CERT").
		Build()

	err = mgr.GetClient().Create(context.TODO(), &configMap)
	assert.NoError(t, err)

	r := newReconciler(mgr, mockManifestProvider(mdb.Spec.Version))
	res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	// Assert that all TLS volumes have been added.
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 6)
	assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "tls",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "tls-ca",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mdb.Spec.Security.TLS.CaConfigMap.Name,
				},
			},
		},
	})
	assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "tls-secret",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: mdb.Spec.Security.TLS.CertificateKeySecret.Name,
			},
		},
	})

	// Assert that the TLS init container has been added.
	tlsVolumeMount := corev1.VolumeMount{
		Name:      "tls",
		ReadOnly:  false,
		MountPath: tlsServerMountPath,
	}
	tlsSecretVolumeMount := corev1.VolumeMount{
		Name:      "tls-secret",
		ReadOnly:  true,
		MountPath: tlsSecretMountPath,
	}
	tlsCAVolumeMount := corev1.VolumeMount{
		Name:      "tls-ca",
		ReadOnly:  true,
		MountPath: tlsCAMountPath,
	}

	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)
	tlsInitContainer := sts.Spec.Template.Spec.InitContainers[1]
	assert.Equal(t, "tls-init", tlsInitContainer.Name)

	// Assert that all containers have the correct volumes mounted.
	assert.Len(t, tlsInitContainer.VolumeMounts, 2)
	assert.Contains(t, tlsInitContainer.VolumeMounts, tlsVolumeMount)
	assert.Contains(t, tlsInitContainer.VolumeMounts, tlsSecretVolumeMount)

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Contains(t, agentContainer.VolumeMounts, tlsVolumeMount)
	assert.Contains(t, agentContainer.VolumeMounts, tlsCAVolumeMount)

	mongodbContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Contains(t, mongodbContainer.VolumeMounts, tlsVolumeMount)
	assert.Contains(t, mongodbContainer.VolumeMounts, tlsCAVolumeMount)
}

func TestAutomationConfig_IsCorrectlyConfiguredWithTLS(t *testing.T) {
	createAC := func(mdb mdbv1.MongoDB) automationconfig.AutomationConfig {
		manifest, err := mockManifestProvider(mdb.Spec.Version)()
		assert.NoError(t, err)
		versionConfig := manifest.BuildsForVersion(mdb.Spec.Version)

		ac, err := buildAutomationConfig(
			mdb,
			versionConfig,
			automationconfig.AutomationConfig{},
			getTLSConfigModification(mdb),
		)
		assert.NoError(t, err)
		return ac
	}

	t.Run("With TLS disabled", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ac := createAC(mdb)

		assert.Equal(t, automationconfig.TLS{
			CAFilePath:            "",
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLS)

		for _, process := range ac.Processes {
			assert.Equal(t, automationconfig.MongoDBTLS{
				Mode: automationconfig.TLSModeDisabled,
			}, process.Args26.Net.TLS)
		}
	})

	t.Run("With TLS enabled, during rollout", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		ac := createAC(mdb)

		assert.Equal(t, automationconfig.TLS{
			CAFilePath:            "",
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLS)

		for _, process := range ac.Processes {
			assert.Equal(t, automationconfig.MongoDBTLS{
				Mode: automationconfig.TLSModeDisabled,
			}, process.Args26.Net.TLS)
		}
	})

	t.Run("With TLS enabled and required, rollout completed", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		mdb.Annotations[tLSRolledOutAnnotationKey] = "true"
		ac := createAC(mdb)

		assert.Equal(t, automationconfig.TLS{
			CAFilePath:            tlsCAMountPath + tlsCACertName,
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLS)

		for _, process := range ac.Processes {
			assert.Equal(t, automationconfig.MongoDBTLS{
				Mode:                               automationconfig.TLSModeRequired,
				PEMKeyFile:                         tlsServerMountPath + tlsServerFileName,
				CAFile:                             tlsCAMountPath + tlsCACertName,
				AllowConnectionsWithoutCertificate: true,
			}, process.Args26.Net.TLS)
		}
	})

	t.Run("With TLS enabled and optional, rollout completed", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		mdb.Annotations[tLSRolledOutAnnotationKey] = "true"
		mdb.Spec.Security.TLS.Optional = true
		ac := createAC(mdb)

		assert.Equal(t, automationconfig.TLS{
			CAFilePath:            tlsCAMountPath + tlsCACertName,
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLS)

		for _, process := range ac.Processes {
			assert.Equal(t, automationconfig.MongoDBTLS{
				Mode:                               automationconfig.TLSModePreferred,
				PEMKeyFile:                         tlsServerMountPath + tlsServerFileName,
				CAFile:                             tlsCAMountPath + tlsCACertName,
				AllowConnectionsWithoutCertificate: true,
			}, process.Args26.Net.TLS)
		}
	})
}

func assertReconciliationSuccessful(t *testing.T, result reconcile.Result, err error) {
	assert.NoError(t, err)
	assert.Equal(t, false, result.Requeue)
	assert.Equal(t, time.Duration(0), result.RequeueAfter)
}

// makeStatefulSetReady updates the StatefulSet corresponding to the
// provided MongoDB resource to mark it as ready for the case of `statefulset.IsReady`
func makeStatefulSetReady(c k8sClient.Client, mdb mdbv1.MongoDB) {
	sts := appsv1.StatefulSet{}
	_ = c.Get(context.TODO(), mdb.NamespacedName(), &sts)
	sts.Status.ReadyReplicas = int32(mdb.Spec.Members)
	sts.Status.UpdatedReplicas = int32(mdb.Spec.Members)
	_ = c.Update(context.TODO(), &sts)
}
