package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"

	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	kubeClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/stretchr/testify/assert"
)

func TestStatefulSetIsCorrectlyConfiguredWithTLS(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mgr := kubeClient.NewManager(ctx, &mdb)

	client := kubeClient.NewClient(mgr.GetClient())
	err := createTLSSecret(ctx, client, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createTLSConfigMap(ctx, client, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, "", "")
}

func TestStatefulSetIsCorrectlyConfiguredWithTLSAndX509(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mdb.Spec.Security.Authentication.Modes = []mdbv1.AuthMode{"X509"}
	mgr := kubeClient.NewManager(ctx, &mdb)

	client := kubeClient.NewClient(mgr.GetClient())
	err := createTLSSecret(ctx, client, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createTLSConfigMap(ctx, client, mdb)
	assert.NoError(t, err)
	crt, key, err := x509.CreateAgentCertificate()
	assert.NoError(t, err)
	err = createAgentCertSecret(ctx, client, mdb, crt, key, "")
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	// Check that the pem secret has been created
	s := corev1.Secret{}
	err = mgr.GetClient().Get(ctx, mdb.AgentCertificatePemSecretNamespacedName(), &s)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, "", mdb.AgentCertificatePemSecretNamespacedName().Name)

	// If we deactivate X509 for the agent, we expect the certificates to be unmounted.
	mdb.Spec.Security.Authentication.Modes = []mdbv1.AuthMode{"SCRAM"}
	err = mgr.GetClient().Update(ctx, &mdb)
	assert.NoError(t, err)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, "", "")
}

func assertStatefulSetVolumesAndVolumeMounts(t *testing.T, sts appsv1.StatefulSet, expectedTLSCASecretName string, expectedTLSOperatorSecretName string, expectedPromTLSSecretName string, expectedAgentCertSecretName string) {
	prometheusTLSEnabled := expectedPromTLSSecretName != ""
	agentX509Enabled := expectedAgentCertSecretName != ""

	permission := int32(416)
	assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "tls-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  expectedTLSCASecretName,
				DefaultMode: &permission,
			},
		},
	})
	assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "tls-secret",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  expectedTLSOperatorSecretName,
				DefaultMode: &permission,
			},
		},
	})
	if prometheusTLSEnabled {
		assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "prom-tls-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  expectedPromTLSSecretName,
					DefaultMode: &permission,
				},
			},
		})
	}
	if agentX509Enabled {
		assert.Contains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "agent-certs-pem",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  expectedAgentCertSecretName,
					DefaultMode: &permission,
				},
			},
		})
	} else {
		assert.NotContains(t, sts.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "agent-certs-pem",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  expectedAgentCertSecretName,
					DefaultMode: &permission,
				},
			},
		})
	}

	tlsSecretVolumeMount := corev1.VolumeMount{
		Name:      "tls-secret",
		ReadOnly:  true,
		MountPath: tlsOperatorSecretMountPath,
	}
	tlsCAVolumeMount := corev1.VolumeMount{
		Name:      "tls-ca",
		ReadOnly:  true,
		MountPath: tlsCAMountPath,
	}
	tlsPrometheusSecretVolumeMount := corev1.VolumeMount{
		Name:      "prom-tls-secret",
		ReadOnly:  true,
		MountPath: tlsPrometheusSecretMountPath,
	}
	agentCertSecretVolumeMount := corev1.VolumeMount{
		Name:      "agent-certs-pem",
		ReadOnly:  true,
		MountPath: automationAgentPemMountPath,
	}

	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)

	var agentContainer corev1.Container
	var mongodbContainer corev1.Container

	for i, container := range sts.Spec.Template.Spec.Containers {
		if container.Name == construct.AgentName {
			agentContainer = sts.Spec.Template.Spec.Containers[i]
		} else if container.Name == construct.MongodbName {
			mongodbContainer = sts.Spec.Template.Spec.Containers[i]
		}
	}

	assert.Contains(t, agentContainer.VolumeMounts, tlsSecretVolumeMount)
	assert.Contains(t, agentContainer.VolumeMounts, tlsCAVolumeMount)
	if prometheusTLSEnabled {
		assert.Contains(t, agentContainer.VolumeMounts, tlsPrometheusSecretVolumeMount)
	}
	if agentX509Enabled {
		assert.Contains(t, agentContainer.VolumeMounts, agentCertSecretVolumeMount)
	} else {
		assert.NotContains(t, agentContainer.VolumeMounts, agentCertSecretVolumeMount)
	}

	assert.Contains(t, mongodbContainer.VolumeMounts, tlsSecretVolumeMount)
	assert.Contains(t, mongodbContainer.VolumeMounts, tlsCAVolumeMount)
	if prometheusTLSEnabled {
		assert.Contains(t, mongodbContainer.VolumeMounts, tlsPrometheusSecretVolumeMount)
	}
}

func TestStatefulSetIsCorrectlyConfiguredWithPrometheusTLS(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mdb.Spec.Prometheus = &mdbv1.Prometheus{
		Username: "username",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "prom-password-secret",
		},
		Port: 4321,
		TLSSecretRef: mdbv1.SecretKeyReference{
			Name: "prom-secret-cert",
		},
	}

	mgr := kubeClient.NewManager(ctx, &mdb)
	cli := kubeClient.NewClient(mgr.GetClient())

	err := secret.CreateOrUpdate(ctx, mgr.Client, secret.Builder().
		SetName("prom-password-secret").
		SetNamespace(mdb.Namespace).
		SetField("password", "my-password").
		Build())
	assert.NoError(t, err)
	err = createTLSSecret(ctx, cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createPrometheusTLSSecret(ctx, cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)

	err = createTLSConfigMap(ctx, cli, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, mdb.PrometheusTLSOperatorSecretNamespacedName().Name, "")
}

func TestStatefulSetIsCorrectlyConfiguredWithTLSAfterChangingExistingVolumes(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mgr := kubeClient.NewManager(ctx, &mdb)

	cli := kubeClient.NewClient(mgr.GetClient())
	err := createTLSSecret(ctx, cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)

	tlsCAVolumeSecretName := mdb.TLSOperatorCASecretNamespacedName().Name
	changedTLSCAVolumeSecretName := tlsCAVolumeSecretName + "-old"

	err = createTLSSecretWithNamespaceAndName(ctx, cli, mdb.Namespace, changedTLSCAVolumeSecretName, "CERT", "KEY", "")
	assert.NoError(t, err)

	err = createTLSConfigMap(ctx, cli, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, tlsCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "", "")

	// updating sts tls-ca volume directly to simulate changing of underlying volume's secret
	for i := range sts.Spec.Template.Spec.Volumes {
		if sts.Spec.Template.Spec.Volumes[i].Name == "tls-ca" {
			sts.Spec.Template.Spec.Volumes[i].VolumeSource.Secret.SecretName = changedTLSCAVolumeSecretName
		}
	}

	err = mgr.GetClient().Update(ctx, &sts)
	assert.NoError(t, err)

	assertStatefulSetVolumesAndVolumeMounts(t, sts, changedTLSCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "", "")

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)
	assertStatefulSetVolumesAndVolumeMounts(t, sts, tlsCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "", "")
}

func TestAutomationConfigIsCorrectlyConfiguredWithTLS(t *testing.T) {
	ctx := context.Background()
	createAC := func(mdb mdbv1.MongoDBCommunity) automationconfig.AutomationConfig {
		client := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, client, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, client, mdb)
		assert.NoError(t, err)

		tlsModification, err := getTLSConfigModification(ctx, client, client, mdb)
		assert.NoError(t, err)
		ac, err := buildAutomationConfig(mdb, false, automationconfig.Auth{}, automationconfig.AutomationConfig{}, tlsModification)
		assert.NoError(t, err)

		return ac
	}

	t.Run("With TLS disabled", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ac := createAC(mdb)

		assert.Equal(t, &automationconfig.TLS{
			CAFilePath:            "",
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLSConfig)

		for _, process := range ac.Processes {
			assert.False(t, process.Args26.Has("net.tls"))
		}
	})

	t.Run("With logRotate and SystemLog enabled", func(t *testing.T) {
		mdb := newTestReplicaSetWithSystemLogAndLogRotate()
		ac := createAC(mdb)

		for _, process := range ac.Processes {
			assert.Equal(t, "/tmp/test", process.Args26.Get("systemLog.path").String())
			assert.Equal(t, "file", process.Args26.Get("systemLog.destination").String())
			assert.Equal(t, process.LogRotate, automationconfig.ConvertCrdLogRotateToAC(mdb.Spec.AgentConfiguration.LogRotate))
			assert.Equal(t, process.AuditLogRotate, automationconfig.ConvertCrdLogRotateToAC(mdb.Spec.AgentConfiguration.AuditLogRotate))
		}
	})

	t.Run("With TLS enabled and required, rollout completed", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		ac := createAC(mdb)

		assert.Equal(t, &automationconfig.TLS{
			CAFilePath:            tlsCAMountPath + tlsOperatorSecretFileName("CERT"),
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLSConfig)

		for _, process := range ac.Processes {
			operatorSecretFileName := tlsOperatorSecretFileName("CERT\nKEY")

			assert.Equal(t, automationconfig.TLSModeRequired, process.Args26.Get("net.tls.mode").Data())
			assert.Equal(t, tlsOperatorSecretMountPath+operatorSecretFileName, process.Args26.Get("net.tls.certificateKeyFile").Data())
			assert.Equal(t, tlsCAMountPath+tlsOperatorSecretFileName("CERT"), process.Args26.Get("net.tls.CAFile").Data())
			assert.True(t, process.Args26.Get("net.tls.allowConnectionsWithoutCertificates").MustBool())
		}
	})

	t.Run("With TLS enabled and optional, rollout completed", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		mdb.Spec.Security.TLS.Optional = true
		ac := createAC(mdb)

		assert.Equal(t, &automationconfig.TLS{
			CAFilePath:            tlsCAMountPath + tlsOperatorSecretFileName("CERT"),
			ClientCertificateMode: automationconfig.ClientCertificateModeOptional,
		}, ac.TLSConfig)

		for _, process := range ac.Processes {
			operatorSecretFileName := tlsOperatorSecretFileName("CERT\nKEY")

			assert.Equal(t, automationconfig.TLSModePreferred, process.Args26.Get("net.tls.mode").Data())
			assert.Equal(t, tlsOperatorSecretMountPath+operatorSecretFileName, process.Args26.Get("net.tls.certificateKeyFile").Data())
			assert.Equal(t, tlsCAMountPath+tlsOperatorSecretFileName("CERT"), process.Args26.Get("net.tls.CAFile").Data())
			assert.True(t, process.Args26.Get("net.tls.allowConnectionsWithoutCertificates").MustBool())
		}
	})
}

func TestTLSOperatorSecret(t *testing.T) {
	ctx := context.Background()
	t.Run("Secret is created if it doesn't exist", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, c, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c), "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

		err = r.ensureTLSResources(ctx, mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(ctx, c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})

	t.Run("Secret is updated if it already exists", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		k8sclient := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, k8sclient, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, k8sclient, mdb)
		assert.NoError(t, err)

		// Create operator-managed secret
		s := secret.Builder().
			SetName(mdb.TLSOperatorSecretNamespacedName().Name).
			SetNamespace(mdb.TLSOperatorSecretNamespacedName().Namespace).
			SetField(tlsOperatorSecretFileName(""), "").
			Build()
		err = k8sclient.CreateSecret(ctx, s)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(k8sclient), "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

		err = r.ensureTLSResources(ctx, mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been updated with the concatenated
		// certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(ctx, k8sclient, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})
}

func TestCombineCertificateAndKey(t *testing.T) {
	tests := []struct {
		Cert     string
		Key      string
		Expected string
	}{
		{"CERT", "KEY", "CERT\nKEY"},
		{"CERT\n", "KEY", "CERT\nKEY"},
		{"CERT", "KEY\n", "CERT\nKEY"},
		{"CERT\n", "KEY\n", "CERT\nKEY"},
		{"CERT\n\n\n", "KEY\n\n\n", "CERT\nKEY"},
	}

	for _, test := range tests {
		combined := combineCertificateAndKey(test.Cert, test.Key)
		assert.Equal(t, test.Expected, combined)
	}
}

func TestPemSupport(t *testing.T) {
	ctx := context.Background()
	t.Run("Success if only pem is provided", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, c, mdb, "", "", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c), "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

		err = r.ensureTLSResources(ctx, mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(ctx, c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})
	t.Run("Success if pem is equal to cert+key", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, c, mdb, "CERT", "KEY", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c), "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

		err = r.ensureTLSResources(ctx, mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(ctx, c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})
	t.Run("Failure if pem is different from cert+key", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := kubeClient.NewClient(kubeClient.NewManager(ctx, &mdb).GetClient())
		err := createTLSSecret(ctx, c, mdb, "CERT1", "KEY1", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(ctx, c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c), "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

		err = r.ensureTLSResources(ctx, mdb)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `if all of "tls.crt", "tls.key" and "tls.pem" are present in the secret, the entry for "tls.pem" must be equal to the concatenation of "tls.crt" with "tls.key"`)
	})
}

func TestTLSConfigReferencesToCACertAreValidated(t *testing.T) {
	ctx := context.Background()
	type args struct {
		caConfigMap         *corev1.LocalObjectReference
		caCertificateSecret *corev1.LocalObjectReference
		expectedError       error
	}
	tests := map[string]args{
		"Success if reference to CA cert provided via secret": {
			caConfigMap: &corev1.LocalObjectReference{
				Name: "certificateKeySecret"},
			caCertificateSecret: nil,
		},
		"Success if reference to CA cert provided via config map": {
			caConfigMap: nil,
			caCertificateSecret: &corev1.LocalObjectReference{
				Name: "caConfigMap"},
		},
		"Succes if reference to CA cert provided both via secret and configMap": {
			caConfigMap: &corev1.LocalObjectReference{
				Name: "certificateKeySecret"},
			caCertificateSecret: &corev1.LocalObjectReference{
				Name: "caConfigMap"},
		},
		"Failure if reference to CA cert is missing": {
			caConfigMap:         nil,
			caCertificateSecret: nil,
			expectedError:       errors.New("TLS field requires a reference to the CA certificate which signed the server certificates. Neither secret (field caCertificateSecretRef) not configMap (field CaConfigMap) reference present"),
		},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			mdb := newTestReplicaSetWithTLSCaCertificateReferences(tc.caConfigMap, tc.caCertificateSecret)

			mgr := kubeClient.NewManager(ctx, &mdb)
			cli := kubeClient.NewClient(mgr.GetClient())
			err := createTLSSecret(ctx, cli, mdb, "cert", "key", "pem")

			assert.NoError(t, err)

			r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", "fake-agentImage", "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

			_, err = r.validateTLSConfig(ctx, mdb)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func createTLSConfigMap(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity) error {
	if !mdb.Spec.Security.TLS.Enabled {
		return nil
	}

	configMap := configmap.Builder().
		SetName(mdb.Spec.Security.TLS.CaConfigMap.Name).
		SetNamespace(mdb.Namespace).
		SetDataField("ca.crt", "CERT").
		Build()

	return c.Create(ctx, &configMap)
}

func createTLSSecretWithNamespaceAndName(ctx context.Context, c k8sClient.Client, namespace string, name string, crt string, key string, pem string) error {
	sBuilder := secret.Builder().
		SetName(name).
		SetNamespace(namespace).
		SetField(tlsCACertName, "CERT")

	if crt != "" {
		sBuilder.SetField(tlsSecretCertName, crt)
	}
	if key != "" {
		sBuilder.SetField(tlsSecretKeyName, key)
	}
	if pem != "" {
		sBuilder.SetField(tlsSecretPemName, pem)
	}

	s := sBuilder.Build()
	return c.Create(ctx, &s)
}

func createTLSSecret(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(ctx, c, mdb.Namespace, mdb.Spec.Security.TLS.CertificateKeySecret.Name, crt, key, pem)
}

func createAgentCertSecret(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(ctx, c, mdb.Namespace, mdb.AgentCertificateSecretNamespacedName().Name, crt, key, pem)
}

func createAgentCertPemSecret(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(ctx, c, mdb.Namespace, mdb.AgentCertificatePemSecretNamespacedName().Name, crt, key, pem)
}

func createPrometheusTLSSecret(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(ctx, c, mdb.Namespace, mdb.Spec.Prometheus.TLSSecretRef.Name, crt, key, pem)
}

func createUserPasswordSecret(ctx context.Context, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, userPasswordSecretName string, password string) error {
	sBuilder := secret.Builder().
		SetName(userPasswordSecretName).
		SetNamespace(mdb.Namespace).
		SetField("password", password)

	s := sBuilder.Build()
	return c.Create(ctx, &s)
}
