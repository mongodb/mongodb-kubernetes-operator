package controllers

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	kubeClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	mdbClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestStatefulSet_IsCorrectlyConfiguredWithTLS(t *testing.T) {
	mdb := newTestReplicaSetWithTLS()
	mgr := kubeClient.NewManager(&mdb)

	client := mdbClient.NewClient(mgr.GetClient())
	err := createTLSSecret(client, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createTLSConfigMap(client, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulsetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, "")
}

func assertStatefulsetVolumesAndVolumeMounts(t *testing.T, sts appsv1.StatefulSet, expectedTLSCASecretName string, expectedTLSOperatorSecretName string, expectedPromTLSSecretName string) {
	prometheusTLSEnabled := expectedPromTLSSecretName != ""

	if prometheusTLSEnabled {
		assert.Len(t, sts.Spec.Template.Spec.Volumes, 9)
	} else {
		assert.Len(t, sts.Spec.Template.Spec.Volumes, 8)
	}
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

	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Contains(t, agentContainer.VolumeMounts, tlsSecretVolumeMount)
	assert.Contains(t, agentContainer.VolumeMounts, tlsCAVolumeMount)
	if prometheusTLSEnabled {
		assert.Contains(t, agentContainer.VolumeMounts, tlsPrometheusSecretVolumeMount)
	}

	mongodbContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Contains(t, mongodbContainer.VolumeMounts, tlsSecretVolumeMount)
	assert.Contains(t, mongodbContainer.VolumeMounts, tlsCAVolumeMount)
	if prometheusTLSEnabled {
		assert.Contains(t, mongodbContainer.VolumeMounts, tlsPrometheusSecretVolumeMount)
	}
}

func TestStatefulSet_IsCorrectlyConfiguredWithPrometheusTLS(t *testing.T) {
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

	mgr := kubeClient.NewManager(&mdb)
	cli := mdbClient.NewClient(mgr.GetClient())

	err := secret.CreateOrUpdate(mgr.Client,
		secret.Builder().
			SetName("prom-password-secret").
			SetNamespace(mdb.Namespace).
			SetField("password", "my-password").
			Build(),
	)
	assert.NoError(t, err)
	err = createTLSSecret(cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createPrometheusTLSSecret(cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)

	err = createTLSConfigMap(cli, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulsetVolumesAndVolumeMounts(t, sts, mdb.TLSOperatorCASecretNamespacedName().Name, mdb.TLSOperatorSecretNamespacedName().Name, mdb.PrometheusTLSOperatorSecretNamespacedName().Name)
}

func TestStatefulSet_IsCorrectlyConfiguredWithTLSAfterChangingExistingVolumes(t *testing.T) {
	mdb := newTestReplicaSetWithTLS()
	mgr := kubeClient.NewManager(&mdb)

	cli := mdbClient.NewClient(mgr.GetClient())
	err := createTLSSecret(cli, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)

	tlsCAVolumeSecretName := mdb.TLSOperatorCASecretNamespacedName().Name
	changedTLSCAVolumeSecretName := tlsCAVolumeSecretName + "-old"

	err = createTLSSecretWithNamespaceAndName(cli, mdb.Namespace, changedTLSCAVolumeSecretName, "CERT", "KEY", "")
	assert.NoError(t, err)

	err = createTLSConfigMap(cli, mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assertStatefulsetVolumesAndVolumeMounts(t, sts, tlsCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "")

	// updating sts tls-ca volume directly to simulate changing of underlying volume's secret
	for i := range sts.Spec.Template.Spec.Volumes {
		if sts.Spec.Template.Spec.Volumes[i].Name == "tls-ca" {
			sts.Spec.Template.Spec.Volumes[i].VolumeSource.Secret.SecretName = changedTLSCAVolumeSecretName
		}
	}

	err = mgr.GetClient().Update(context.TODO(), &sts)
	assert.NoError(t, err)

	assertStatefulsetVolumesAndVolumeMounts(t, sts, changedTLSCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "")

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)
	assertStatefulsetVolumesAndVolumeMounts(t, sts, tlsCAVolumeSecretName, mdb.TLSOperatorSecretNamespacedName().Name, "")
}

func TestAutomationConfig_IsCorrectlyConfiguredWithTLS(t *testing.T) {
	createAC := func(mdb mdbv1.MongoDBCommunity) automationconfig.AutomationConfig {
		client := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(client, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(client, mdb)
		assert.NoError(t, err)

		tlsModification, err := getTLSConfigModification(client, client, mdb)
		assert.NoError(t, err)
		ac, err := buildAutomationConfig(mdb, automationconfig.Auth{}, automationconfig.AutomationConfig{}, tlsModification)
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
	t.Run("Secret is created if it doesn't exist", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(c, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c))

		err = r.ensureTLSResources(mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})

	t.Run("Secret is updated if it already exists", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		k8sclient := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(k8sclient, mdb, "CERT", "KEY", "")
		assert.NoError(t, err)
		err = createTLSConfigMap(k8sclient, mdb)
		assert.NoError(t, err)

		// Create operator-managed secret
		s := secret.Builder().
			SetName(mdb.TLSOperatorSecretNamespacedName().Name).
			SetNamespace(mdb.TLSOperatorSecretNamespacedName().Namespace).
			SetField(tlsOperatorSecretFileName(""), "").
			Build()
		err = k8sclient.CreateSecret(s)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(k8sclient))

		err = r.ensureTLSResources(mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been updated with the concatenated
		// certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(k8sclient, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
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
	t.Run("Success if only pem is provided", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(c, mdb, "", "", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c))

		err = r.ensureTLSResources(mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})
	t.Run("Success if pem is equal to cert+key", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(c, mdb, "CERT", "KEY", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c))

		err = r.ensureTLSResources(mdb)
		assert.NoError(t, err)

		// Operator-managed secret should have been created and contains the
		// concatenated certificate and key.
		expectedCertificateKey := "CERT\nKEY"
		certificateKey, err := secret.ReadKey(c, tlsOperatorSecretFileName(expectedCertificateKey), mdb.TLSOperatorSecretNamespacedName())
		assert.NoError(t, err)
		assert.Equal(t, expectedCertificateKey, certificateKey)
	})
	t.Run("Failure if pem is different from cert+key", func(t *testing.T) {
		mdb := newTestReplicaSetWithTLS()
		c := mdbClient.NewClient(kubeClient.NewManager(&mdb).GetClient())
		err := createTLSSecret(c, mdb, "CERT1", "KEY1", "CERT\nKEY")
		assert.NoError(t, err)
		err = createTLSConfigMap(c, mdb)
		assert.NoError(t, err)

		r := NewReconciler(kubeClient.NewManagerWithClient(c))

		err = r.ensureTLSResources(mdb)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `if all of "tls.crt", "tls.key" and "tls.pem" are present in the secret, the entry for "tls.pem" must be equal to the concatenation of "tls.crt" with "tls.key"`)
	})
}

func TestTLSConfig_ReferencesToCACertAreValidated(t *testing.T) {
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

			mgr := kubeClient.NewManager(&mdb)
			cli := mdbClient.NewClient(mgr.GetClient())
			err := createTLSSecret(cli, mdb, "cert", "key", "pem")

			assert.NoError(t, err)

			r := NewReconciler(mgr)

			_, err = r.validateTLSConfig(mdb)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func createTLSConfigMap(c k8sClient.Client, mdb mdbv1.MongoDBCommunity) error {
	if !mdb.Spec.Security.TLS.Enabled {
		return nil
	}

	configMap := configmap.Builder().
		SetName(mdb.Spec.Security.TLS.CaConfigMap.Name).
		SetNamespace(mdb.Namespace).
		SetDataField("ca.crt", "CERT").
		Build()

	return c.Create(context.TODO(), &configMap)
}

func createTLSSecretWithNamespaceAndName(c k8sClient.Client, namespace string, name string, crt string, key string, pem string) error {
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
	return c.Create(context.TODO(), &s)
}

func createTLSSecret(c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(c, mdb.Namespace, mdb.Spec.Security.TLS.CertificateKeySecret.Name, crt, key, pem)
}

func createPrometheusTLSSecret(c k8sClient.Client, mdb mdbv1.MongoDBCommunity, crt string, key string, pem string) error {
	return createTLSSecretWithNamespaceAndName(c, mdb.Namespace, mdb.Spec.Prometheus.TLSSecretRef.Name, crt, key, pem)
}

func createUserPasswordSecret(c k8sClient.Client, mdb mdbv1.MongoDBCommunity, userPasswordSecretName string, password string) error {
	sBuilder := secret.Builder().
		SetName(userPasswordSecretName).
		SetNamespace(mdb.Namespace).
		SetField("password", password)

	s := sBuilder.Build()
	return c.Create(context.TODO(), &s)
}
