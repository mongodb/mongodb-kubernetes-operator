package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/stretchr/testify/require"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/objx"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	AgentImage = "fake-agentImage"
)

func newTestReplicaSet() mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "6.0.5",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
		},
	}
}

func newTestReplicaSetWithSystemLogAndLogRotate() mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "6.0.5",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			AgentConfiguration: mdbv1.AgentConfiguration{
				LogRotate: &automationconfig.CrdLogRotate{
					SizeThresholdMB: "1",
				},
				AuditLogRotate: &automationconfig.CrdLogRotate{
					SizeThresholdMB: "1",
				},
				SystemLog: &automationconfig.SystemLog{
					Destination: automationconfig.File,
					Path:        "/tmp/test",
				},
			},
		},
	}
}

func newScramReplicaSet(users ...mdbv1.MongoDBUser) mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Users:   users,
			Members: 3,
			Version: "4.2.2",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
		},
	}
}

func newTestReplicaSetWithTLS() mdbv1.MongoDBCommunity {
	return newTestReplicaSetWithTLSCaCertificateReferences(&corev1.LocalObjectReference{
		Name: "caConfigMap",
	},
		&corev1.LocalObjectReference{
			Name: "certificateKeySecret",
		})
}

func newTestReplicaSetWithTLSCaCertificateReferences(caConfigMap, caCertificateSecret *corev1.LocalObjectReference) mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
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
					Enabled:             true,
					CaConfigMap:         caConfigMap,
					CaCertificateSecret: caCertificateSecret,
					CertificateKeySecret: corev1.LocalObjectReference{
						Name: "certificateKeySecret",
					},
				},
			},
		},
	}
}

func TestKubernetesResources_AreCreated(t *testing.T) {
	ctx := context.Background()
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	s := corev1.Secret{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace}, &s)
	assert.NoError(t, err)
	assert.Equal(t, mdb.Namespace, s.Namespace)
	assert.Equal(t, mdb.AutomationConfigSecretName(), s.Name)
	assert.Contains(t, s.Data, automationconfig.ConfigKey)
	assert.NotEmpty(t, s.Data[automationconfig.ConfigKey])
}

func TestStatefulSet_IsCorrectlyConfigured(t *testing.T) {
	ctx := context.Background()

	mdb := newTestReplicaSet()
	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "docker.io/mongodb", "mongodb-community-server", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)

	agentContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, construct.AgentName, agentContainer.Name)
	assert.Equal(t, AgentImage, agentContainer.Image)
	expectedProbe := probes.New(construct.DefaultReadiness())
	assert.True(t, reflect.DeepEqual(&expectedProbe, agentContainer.ReadinessProbe))

	mongodbContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, construct.MongodbName, mongodbContainer.Name)
	assert.Equal(t, "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8", mongodbContainer.Image)

	assert.Equal(t, resourcerequirements.Defaults(), agentContainer.Resources)

	acVolume, err := getVolumeByName(sts, "automation-config")
	assert.NoError(t, err)
	assert.NotNil(t, acVolume.Secret, "automation config should be stored in a secret!")
	assert.Nil(t, acVolume.ConfigMap, "automation config should be stored in a secret, not a config map!")
}

func TestGuessEnterprise(t *testing.T) {
	type testConfig struct {
		setArgs            func(t *testing.T)
		mdb                mdbv1.MongoDBCommunity
		mongodbImage       string
		expectedEnterprise bool
	}
	tests := map[string]testConfig{
		"No override and Community image": {
			setArgs:            func(t *testing.T) {},
			mdb:                mdbv1.MongoDBCommunity{},
			mongodbImage:       "mongodb-community-server",
			expectedEnterprise: false,
		},
		"No override and Enterprise image": {
			setArgs:            func(t *testing.T) {},
			mdb:                mdbv1.MongoDBCommunity{},
			mongodbImage:       "mongodb-enterprise-server",
			expectedEnterprise: true,
		},
		"Assuming enterprise manually": {
			setArgs: func(t *testing.T) {
				t.Setenv(construct.MongoDBAssumeEnterpriseEnv, "true")
			},
			mdb:                mdbv1.MongoDBCommunity{},
			mongodbImage:       "mongodb-community-server",
			expectedEnterprise: true,
		},
		"Assuming community manually": {
			setArgs: func(t *testing.T) {
				t.Setenv(construct.MongoDBAssumeEnterpriseEnv, "false")
			},
			mdb:                mdbv1.MongoDBCommunity{},
			mongodbImage:       "mongodb-enterprise-server",
			expectedEnterprise: false,
		},
		// This one is a corner case. We don't expect users to fall here very often as there are
		// dedicated variables to control this type of behavior.
		"Enterprise with StatefulSet override": {
			setArgs: func(t *testing.T) {},
			mdb: mdbv1.MongoDBCommunity{
				Spec: mdbv1.MongoDBCommunitySpec{
					StatefulSetConfiguration: mdbv1.StatefulSetConfiguration{
						SpecWrapper: mdbv1.StatefulSetSpecWrapper{
							Spec: appsv1.StatefulSetSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:  construct.MongodbName,
												Image: "another_repo.com/another_org/mongodb-enterprise-server",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			mongodbImage:       "mongodb-community-server",
			expectedEnterprise: true,
		},
		"Enterprise with StatefulSet override to Community": {
			setArgs: func(t *testing.T) {},
			mdb: mdbv1.MongoDBCommunity{
				Spec: mdbv1.MongoDBCommunitySpec{
					StatefulSetConfiguration: mdbv1.StatefulSetConfiguration{
						SpecWrapper: mdbv1.StatefulSetSpecWrapper{
							Spec: appsv1.StatefulSetSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:  construct.MongodbName,
												Image: "another_repo.com/another_org/mongodb-community-server",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			mongodbImage:       "mongodb-enterprise-server",
			expectedEnterprise: false,
		},
	}
	for testName := range tests {
		t.Run(testName, func(t *testing.T) {
			testConfig := tests[testName]
			testConfig.setArgs(t)
			calculatedEnterprise := guessEnterprise(testConfig.mdb, testConfig.mongodbImage)
			assert.Equal(t, testConfig.expectedEnterprise, calculatedEnterprise)
		})
	}
}

func getVolumeByName(sts appsv1.StatefulSet, volumeName string) (corev1.Volume, error) {
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == volumeName {
			return v, nil
		}
	}
	return corev1.Volume{}, fmt.Errorf("volume with name %s, not found", volumeName)
}

func TestChangingVersion_ResultsInRollingUpdateStrategyType(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	mgr := client.NewManager(ctx, &mdb)
	mgrClient := mgr.GetClient()
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)

	// fetch updated resource after first reconciliation
	_ = mgrClient.Get(ctx, mdb.NamespacedName(), &mdb)

	sts := appsv1.StatefulSet{}
	err = mgrClient.Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)
	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)

	mdbRef := &mdb
	mdbRef.Spec.Version = "4.2.3"

	_ = mgrClient.Update(ctx, &mdb)

	// agents start the upgrade, they are not all ready
	sts.Status.UpdatedReplicas = 1
	sts.Status.ReadyReplicas = 2
	err = mgrClient.Update(ctx, &sts)
	assert.NoError(t, err)
	_ = mgrClient.Get(ctx, mdb.NamespacedName(), &sts)

	// reconcilliation is successful
	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts = appsv1.StatefulSet{}
	err = mgrClient.Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type,
		"The StatefulSet should have be re-configured to use RollingUpdates after it reached the ready state")
}

func TestBuildStatefulSet_ConfiguresUpdateStrategyCorrectly(t *testing.T) {
	t.Run("On No Version Change, Same Version", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		mdb.Annotations[annotations.LastAppliedMongoDBVersion] = "4.0.0"
		sts := appsv1.StatefulSet{}
		buildStatefulSetModificationFunction(mdb, "fake-mongodbImage", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")(&sts)
		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
	t.Run("On No Version Change, First Version", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		delete(mdb.Annotations, annotations.LastAppliedMongoDBVersion)
		sts := appsv1.StatefulSet{}
		buildStatefulSetModificationFunction(mdb, "fake-mongodbImage", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")(&sts)
		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
	t.Run("On Version Change", func(t *testing.T) {
		mdb := newTestReplicaSet()

		mdb.Spec.Version = "4.0.0"

		prevSpec := mdbv1.MongoDBCommunitySpec{
			Version: "4.2.0",
		}

		bytes, err := json.Marshal(prevSpec)
		assert.NoError(t, err)

		mdb.Annotations[annotations.LastAppliedMongoDBVersion] = string(bytes)
		sts := appsv1.StatefulSet{}
		buildStatefulSetModificationFunction(mdb, "fake-mongodbImage", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")(&sts)
		assert.Equal(t, appsv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
}

func TestService_isCorrectlyCreatedAndUpdated(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 27017, Name: "mongodb"})

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func TestService_usesCustomMongodPortWhenSpecified(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mongodConfig := objx.New(map[string]interface{}{})
	mongodConfig.Set("net.port", 1000.)
	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 1000, Name: "mongodb"})

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func createOrUpdatePodsWithVersions(ctx context.Context, t *testing.T, c k8sClient.Client, name types.NamespacedName, versions []string) {
	for i, version := range versions {
		createPodWithAgentAnnotation(ctx, t, c, types.NamespacedName{
			Namespace: name.Namespace,
			Name:      fmt.Sprintf("%s-%d", name.Name, i),
		}, version)
	}
}

func createPodWithAgentAnnotation(ctx context.Context, t *testing.T, c k8sClient.Client, name types.NamespacedName, versionStr string) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Annotations: map[string]string{
				"agent.mongodb.com/version": versionStr,
			},
		},
	}

	err := c.Create(ctx, &pod)

	if err != nil && apiErrors.IsAlreadyExists(err) {
		err = c.Update(ctx, &pod)
		assert.NoError(t, err)
	}

	assert.NoError(t, err)
}

func TestService_changesMongodPortOnRunningClusterWithArbiters(t *testing.T) {
	ctx := context.Background()
	mdb := newScramReplicaSet(mdbv1.MongoDBUser{
		Name: "testuser",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "password-secret-name",
		},
		ScramCredentialsSecretName: "scram-credentials",
	})

	namespacedName := mdb.NamespacedName()
	arbiterNamespacedName := mdb.ArbiterNamespacedName()

	const oldPort = automationconfig.DefaultDBPort
	const newPort = 8000

	mgr := client.NewManager(ctx, &mdb)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")

	t.Run("Prepare cluster with arbiters and change port", func(t *testing.T) {
		err := createUserPasswordSecret(ctx, mgr.Client, mdb, "password-secret-name", "pass")
		assert.NoError(t, err)

		mdb.Spec.Arbiters = 1
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		assertReconciliationSuccessful(t, res, err)
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
		})
		_ = assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 1)

		setStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), namespacedName, []string{"1", "1", "1"})
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), arbiterNamespacedName, []string{"1"})

		res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		assertReconciliationSuccessful(t, res, err)
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
		})
		_ = assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 1)
		assertStatefulsetReady(ctx, t, mgr, namespacedName, 3)
		assertStatefulsetReady(ctx, t, mgr, arbiterNamespacedName, 1)

		mdb.Spec.AdditionalMongodConfig = mdbv1.NewMongodConfiguration()
		mdb.Spec.AdditionalMongodConfig.SetDBPort(newPort)

		err = mgr.GetClient().Update(ctx, &mdb)
		assert.NoError(t, err)

		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Port should be changed only in the process #0", func(t *testing.T) {
		// port changes should be performed one at a time
		// should set port #0 to new one
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)

		currentAc := assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 2)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in processes #0,#1", func(t *testing.T) {
		setStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), namespacedName, []string{"2", "2", "2"})
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), arbiterNamespacedName, []string{"2"})

		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 3)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in processes #0,#1,#2", func(t *testing.T) {
		setStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), namespacedName, []string{"3", "3", "3"})
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), arbiterNamespacedName, []string{"3"})

		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 4)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in all processes", func(t *testing.T) {
		setStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), namespacedName, []string{"4", "4", "4"})
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), arbiterNamespacedName, []string{"4"})

		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
		assert.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 5)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[3].GetPort())

		// all the ports are changed but there are still two service ports for old and new port until the next reconcile
		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("At the end there should be only new port in the service", func(t *testing.T) {
		setStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(ctx, t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), namespacedName, []string{"5", "5", "5"})
		createOrUpdatePodsWithVersions(ctx, t, mgr.GetClient(), arbiterNamespacedName, []string{"5"})

		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
		assert.NoError(t, err)
		// no need to requeue, port change is finished
		assert.False(t, res.Requeue)
		// there should not be any changes in config anymore
		currentAc := assertAutomationConfigVersion(ctx, t, mgr.Client, mdb, 5)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[3].GetPort())

		assertServicePorts(ctx, t, mgr.Client, mdb, map[int]string{
			newPort: "mongodb",
		})

		// only at the end, when all pods are ready we have updated connection strings
		assertConnectionStringSecretPorts(ctx, t, mgr.GetClient(), mdb, newPort, oldPort)
	})
}

// assertConnectionStringSecretPorts checks that connection string secret has expectedPort and does not have notExpectedPort.
func assertConnectionStringSecretPorts(ctx context.Context, t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, expectedPort int, notExpectedPort int) {
	connectionStringSecret := corev1.Secret{}
	scramUsers := mdb.GetAuthUsers()
	require.Len(t, scramUsers, 1)
	secretNamespacedName := types.NamespacedName{Name: scramUsers[0].ConnectionStringSecretName, Namespace: scramUsers[0].ConnectionStringSecretNamespace}
	err := c.Get(ctx, secretNamespacedName, &connectionStringSecret)
	require.NoError(t, err)
	require.Contains(t, connectionStringSecret.Data, "connectionString.standard")
	assert.Contains(t, string(connectionStringSecret.Data["connectionString.standard"]), fmt.Sprintf("%d", expectedPort))
	assert.NotContains(t, string(connectionStringSecret.Data["connectionString.standard"]), fmt.Sprintf("%d", notExpectedPort))
}

func assertServicePorts(ctx context.Context, t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, expectedServicePorts map[int]string) {
	svc := corev1.Service{}

	err := c.Get(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	require.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(t, mdb.ServiceName(), svc.Spec.Selector["app"])
	assert.Len(t, svc.Spec.Ports, len(expectedServicePorts))

	actualServicePorts := map[int]string{}
	for _, servicePort := range svc.Spec.Ports {
		actualServicePorts[int(servicePort.Port)] = servicePort.Name
	}

	assert.Equal(t, expectedServicePorts, actualServicePorts)
}

func assertAutomationConfigVersion(ctx context.Context, t *testing.T, c client.Client, mdb mdbv1.MongoDBCommunity, expectedVersion int) automationconfig.AutomationConfig {
	ac, err := automationconfig.ReadFromSecret(ctx, c, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, ac.Version)
	return ac
}

func assertStatefulsetReady(ctx context.Context, t *testing.T, mgr manager.Manager, name types.NamespacedName, expectedReplicas int) {
	sts := appsv1.StatefulSet{}
	err := mgr.GetClient().Get(ctx, name, &sts)
	require.NoError(t, err)
	assert.True(t, statefulset.IsReady(sts, expectedReplicas))
}

func TestService_configuresPrometheusCustomPorts(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	mdb.Spec.Prometheus = &mdbv1.Prometheus{
		Username: "username",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "secret",
		},
		Port: 4321,
	}

	mongodConfig := objx.New(map[string]interface{}{})
	mongodConfig.Set("net.port", 1000.)
	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	mgr := client.NewManager(ctx, &mdb)
	err := secret.CreateOrUpdate(ctx, mgr.Client, secret.Builder().
		SetName("secret").
		SetNamespace(mdb.Namespace).
		SetField("password", "my-password").
		Build())

	assert.NoError(t, err)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 1000, Name: "mongodb"})
	assert.Equal(t, svc.Spec.Ports[1], corev1.ServicePort{Port: 4321, Name: "prometheus"})

	assert.Equal(t, svc.Labels["app"], mdb.ServiceName())

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func TestService_configuresPrometheus(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	mdb.Spec.Prometheus = &mdbv1.Prometheus{
		Username: "username",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "secret",
		},
	}

	mgr := client.NewManager(ctx, &mdb)
	err := secret.CreateOrUpdate(ctx, mgr.Client, secret.Builder().
		SetName("secret").
		SetNamespace(mdb.Namespace).
		SetField("password", "my-password").
		Build())
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)

	assert.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 27017, Name: "mongodb"})
	assert.Equal(t, svc.Spec.Ports[1], corev1.ServicePort{Port: 9216, Name: "prometheus"})
}

func TestCustomNetPort_Configuration(t *testing.T) {
	ctx := context.Background()
	svc, _ := performReconciliationAndGetService(ctx, t, "specify_net_port.yaml")
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, corev1.ServicePort{Port: 40333, Name: "mongodb"}, svc.Spec.Ports[0])
}

func TestAutomationConfig_versionIsBumpedOnChange(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, 1, currentAc.Version)

	mdb.Spec.Members++
	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	_ = mgr.GetClient().Update(ctx, &mdb)
	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, 2, currentAc.Version)
}

func TestAutomationConfig_versionIsNotBumpedWithNoChanges(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)
}

func TestAutomationConfigFCVIsNotIncreasedWhenUpgradingMinorVersion(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	mdb.Spec.Version = "4.2.2"
	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Len(t, currentAc.Processes, 3)
	assert.Equal(t, currentAc.Processes[0].FeatureCompatibilityVersion, "4.2")

	// Upgrading minor version does not change the FCV on the automationConfig
	mdbRef := &mdb
	mdbRef.Spec.Version = "4.4.0"
	_ = mgr.Client.Update(ctx, mdbRef)
	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Len(t, currentAc.Processes, 3)
	assert.Equal(t, currentAc.Processes[0].FeatureCompatibilityVersion, "4.2")

}

func TestAutomationConfig_CustomMongodConfig(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mongodConfig := objx.New(map[string]interface{}{})
	mongodConfig.Set("net.port", float64(1000))
	mongodConfig.Set("storage.other", "value")
	mongodConfig.Set("arbitrary.config.path", "value")
	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)

	for _, p := range currentAc.Processes {
		// Ensure port was overridden
		assert.Equal(t, float64(1000), p.Args26.Get("net.port").Data())

		// Ensure custom values were added
		assert.Equal(t, "value", p.Args26.Get("arbitrary.config.path").Data())
		assert.Equal(t, "value", p.Args26.Get("storage.other").Data())

		// Ensure default settings went unchanged
		assert.Equal(t, automationconfig.DefaultMongoDBDataDir, p.Args26.Get("storage.dbPath").Data())
		assert.Equal(t, mdb.Name, p.Args26.Get("replication.replSetName").Data())
	}
}

func TestExistingPasswordAndKeyfile_AreUsedWhenTheSecretExists(t *testing.T) {
	ctx := context.Background()
	mdb := newScramReplicaSet()
	mgr := client.NewManager(ctx, &mdb)

	c := mgr.Client

	keyFileNsName := mdb.GetAgentKeyfileSecretNamespacedName()
	err := secret.CreateOrUpdate(ctx, c, secret.Builder().
		SetName(keyFileNsName.Name).
		SetNamespace(keyFileNsName.Namespace).
		SetField(constants.AgentKeyfileKey, "my-keyfile").
		Build())
	assert.NoError(t, err)

	passwordNsName := mdb.GetAgentPasswordSecretNamespacedName()
	err = secret.CreateOrUpdate(ctx, c, secret.Builder().
		SetName(passwordNsName.Name).
		SetNamespace(passwordNsName.Namespace).
		SetField(constants.AgentPasswordKey, "my-pass").
		Build())
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.False(t, currentAc.Auth.Disabled)

	assert.Equal(t, "my-keyfile", currentAc.Auth.Key)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.Equal(t, "my-pass", currentAc.Auth.AutoPwd)

}

func TestScramIsConfigured(t *testing.T) {
	ctx := context.Background()
	assertReplicaSetIsConfiguredWithScram(ctx, t, newScramReplicaSet())
}

func TestScramIsConfiguredWhenNotSpecified(t *testing.T) {
	ctx := context.Background()
	assertReplicaSetIsConfiguredWithScram(ctx, t, newTestReplicaSet())
}

func TestReplicaSet_IsScaledDown_OneMember_AtATime_WhenItAlreadyExists(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	mdb.Spec.Members = 5

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)

	assert.NoError(t, err)
	assert.Equal(t, 5, mdb.Status.CurrentMongoDBMembers)

	// scale members from five to three
	mdb.Spec.Members = 3

	err = mgr.GetClient().Update(ctx, &mdb)
	assert.NoError(t, err)

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)
	assert.NoError(t, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, true, res.Requeue)
	assert.Equal(t, 4, mdb.Status.CurrentMongoDBMembers)

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)
	assert.Equal(t, false, res.Requeue)
	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)
}

func TestReplicaSet_IsScaledUp_OneMember_AtATime_WhenItAlreadyExists(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)
	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)

	// scale members from three to five
	mdb.Spec.Members = 5

	err = mgr.GetClient().Update(ctx, &mdb)
	assert.NoError(t, err)

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)

	assert.NoError(t, err)
	assert.Equal(t, true, res.Requeue)
	assert.Equal(t, 4, mdb.Status.CurrentMongoDBMembers)

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	makeStatefulSetReady(ctx, t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, false, res.Requeue)
	assert.Equal(t, 5, mdb.Status.CurrentMongoDBMembers)
}

func TestIgnoreUnknownUsers(t *testing.T) {
	ctx := context.Background()
	t.Run("Ignore Unkown Users set to true", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ignoreUnknownUsers := true
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = &ignoreUnknownUsers

		assertAuthoritativeSet(ctx, t, mdb, false)
	})

	t.Run("IgnoreUnknownUsers is not set", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = nil
		assertAuthoritativeSet(ctx, t, mdb, false)
	})

	t.Run("IgnoreUnknownUsers set to false", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ignoreUnknownUsers := false
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = &ignoreUnknownUsers
		assertAuthoritativeSet(ctx, t, mdb, true)
	})
}

func TestAnnotationsAreAppliedToResource(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.NotNil(t, mdb.Annotations)
	assert.NotEmpty(t, mdb.Annotations[lastSuccessfulConfiguration], "last successful spec should have been saved as annotation but was not")
	assert.Equal(t, mdb.Annotations[lastAppliedMongoDBVersion], mdb.Spec.Version, "last version should have been saved as an annotation but was not")
}

// assertAuthoritativeSet asserts that a reconciliation of the given MongoDBCommunity resource
// results in the AuthoritativeSet of the created AutomationConfig to have the expectedValue provided.
func assertAuthoritativeSet(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity, expectedValue bool) {
	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	s, err := mgr.Client.GetSecret(ctx, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)

	bytes := s.Data[automationconfig.ConfigKey]
	ac, err := automationconfig.FromBytes(bytes)
	assert.NoError(t, err)

	assert.Equal(t, expectedValue, ac.Auth.AuthoritativeSet)
}

func assertReplicaSetIsConfiguredWithScram(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity) {
	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	t.Run("Automation Config is configured with SCRAM", func(t *testing.T) {
		assert.NotEmpty(t, currentAc.Auth.Key)
		assert.NoError(t, err)
		assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
		assert.NotEmpty(t, currentAc.Auth.AutoPwd)
		assert.False(t, currentAc.Auth.Disabled)
	})
	t.Run("Secret with password was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentPasswordSecretNamespacedName()
		s, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[constants.AgentPasswordKey], []byte(currentAc.Auth.AutoPwd))
	})

	t.Run("Secret with keyfile was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentKeyfileSecretNamespacedName()
		s, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[constants.AgentKeyfileKey], []byte(currentAc.Auth.Key))
	})
}

func assertReplicaSetIsConfiguredWithScramTLS(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity) {
	mgr := client.NewManager(ctx, &mdb)
	newClient := client.NewClient(mgr.GetClient())
	err := createTLSSecret(ctx, newClient, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createTLSConfigMap(ctx, newClient, mdb)
	assert.NoError(t, err)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	t.Run("Automation Config is configured with SCRAM", func(t *testing.T) {
		assert.Empty(t, currentAc.TLSConfig.AutoPEMKeyFilePath)
		assert.NotEmpty(t, currentAc.Auth.Key)
		assert.NoError(t, err)
		assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
		assert.NotEmpty(t, currentAc.Auth.AutoPwd)
		assert.False(t, currentAc.Auth.Disabled)
	})
	t.Run("Secret with password was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentPasswordSecretNamespacedName()
		s, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[constants.AgentPasswordKey], []byte(currentAc.Auth.AutoPwd))
	})

	t.Run("Secret with keyfile was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentKeyfileSecretNamespacedName()
		s, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[constants.AgentKeyfileKey], []byte(currentAc.Auth.Key))
	})
}

func assertReplicaSetIsConfiguredWithX509(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity) {
	mgr := client.NewManager(ctx, &mdb)
	newClient := client.NewClient(mgr.GetClient())
	err := createTLSSecret(ctx, newClient, mdb, "CERT", "KEY", "")
	assert.NoError(t, err)
	err = createTLSConfigMap(ctx, newClient, mdb)
	assert.NoError(t, err)
	crt, key, err := x509.CreateAgentCertificate()
	assert.NoError(t, err)
	err = createAgentCertSecret(ctx, newClient, mdb, crt, key, "")
	assert.NoError(t, err)

	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(ctx, mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})

	t.Run("Automation Config is configured with X509", func(t *testing.T) {
		assert.NotEmpty(t, currentAc.TLSConfig.AutoPEMKeyFilePath)
		assert.Equal(t, automationAgentPemMountPath+"/"+mdb.AgentCertificatePemSecretNamespacedName().Name, currentAc.TLSConfig.AutoPEMKeyFilePath)
		assert.NotEmpty(t, currentAc.Auth.Key)
		assert.NoError(t, err)
		assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
		assert.Empty(t, currentAc.Auth.AutoPwd)
		assert.False(t, currentAc.Auth.Disabled)
		assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB,C=US", currentAc.Auth.AutoUser)
	})
	t.Run("Secret with password was not created", func(t *testing.T) {
		secretNsName := mdb.GetAgentPasswordSecretNamespacedName()
		_, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.Error(t, err)
	})
	t.Run("Secret with keyfile was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentKeyfileSecretNamespacedName()
		s, err := mgr.Client.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[constants.AgentKeyfileKey], []byte(currentAc.Auth.Key))
	})
}

func TestX509andSCRAMIsConfiguredWithX509Agent(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mdb.Spec.Security.Authentication.Modes = []mdbv1.AuthMode{"X509", "SCRAM"}
	mdb.Spec.Security.Authentication.AgentMode = "X509"

	assertReplicaSetIsConfiguredWithX509(ctx, t, mdb)
}

func TestX509andSCRAMIsConfiguredWithSCRAMAgent(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mdb.Spec.Security.Authentication.Modes = []mdbv1.AuthMode{"X509", "SCRAM"}
	mdb.Spec.Security.Authentication.AgentMode = "SCRAM"

	assertReplicaSetIsConfiguredWithScramTLS(ctx, t, mdb)
}

func TestX509IsConfigured(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSetWithTLS()
	mdb.Spec.Security.Authentication.Modes = []mdbv1.AuthMode{"X509"}

	assertReplicaSetIsConfiguredWithX509(ctx, t, mdb)
}

func TestReplicaSet_IsScaledUpToDesiredMembers_WhenFirstCreated(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(ctx, mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)
}

func TestVolumeClaimTemplates_Configuration(t *testing.T) {
	ctx := context.Background()
	sts, _ := performReconciliationAndGetStatefulSet(ctx, t, "volume_claim_templates_mdb.yaml")

	assert.Len(t, sts.Spec.VolumeClaimTemplates, 3)

	pvcSpec := sts.Spec.VolumeClaimTemplates[2].Spec

	storage := pvcSpec.Resources.Requests[corev1.ResourceStorage]
	storageRef := &storage

	assert.Equal(t, "1Gi", storageRef.String())
	assert.Len(t, pvcSpec.AccessModes, 1)
	assert.Contains(t, pvcSpec.AccessModes, corev1.ReadWriteOnce)
}

func TestChangeDataVolume_Configuration(t *testing.T) {
	ctx := context.Background()
	sts, _ := performReconciliationAndGetStatefulSet(ctx, t, "change_data_volume.yaml")
	assert.Len(t, sts.Spec.VolumeClaimTemplates, 2)

	dataVolume := sts.Spec.VolumeClaimTemplates[0]

	storage := dataVolume.Spec.Resources.Requests[corev1.ResourceStorage]
	storageRef := &storage

	assert.Equal(t, "data-volume", dataVolume.Name)
	assert.Equal(t, "50Gi", storageRef.String())
}

func TestCustomStorageClass_Configuration(t *testing.T) {
	ctx := context.Background()
	sts, _ := performReconciliationAndGetStatefulSet(ctx, t, "custom_storage_class.yaml")

	dataVolume := sts.Spec.VolumeClaimTemplates[0]

	storage := dataVolume.Spec.Resources.Requests[corev1.ResourceStorage]
	storageRef := &storage

	expectedStorageClass := "my-storage-class"
	expectedStorageClassRef := &expectedStorageClass

	assert.Equal(t, "data-volume", dataVolume.Name)
	assert.Equal(t, "1Gi", storageRef.String())
	assert.Equal(t, expectedStorageClassRef, dataVolume.Spec.StorageClassName)
}

func TestCustomTaintsAndTolerations_Configuration(t *testing.T) {
	ctx := context.Background()
	sts, _ := performReconciliationAndGetStatefulSet(ctx, t, "tolerations_example.yaml")

	assert.Len(t, sts.Spec.Template.Spec.Tolerations, 2)
	assert.Equal(t, "example-key", sts.Spec.Template.Spec.Tolerations[0].Key)
	assert.Equal(t, corev1.TolerationOpExists, sts.Spec.Template.Spec.Tolerations[0].Operator)
	assert.Equal(t, corev1.TaintEffectNoSchedule, sts.Spec.Template.Spec.Tolerations[0].Effect)

	assert.Equal(t, "example-key-2", sts.Spec.Template.Spec.Tolerations[1].Key)
	assert.Equal(t, corev1.TolerationOpEqual, sts.Spec.Template.Spec.Tolerations[1].Operator)
	assert.Equal(t, corev1.TaintEffectNoExecute, sts.Spec.Template.Spec.Tolerations[1].Effect)
}

func TestCustomDataDir_Configuration(t *testing.T) {
	ctx := context.Background()
	sts, c := performReconciliationAndGetStatefulSet(ctx, t, "specify_data_dir.yaml")

	agentContainer := container.GetByName("mongodb-agent", sts.Spec.Template.Spec.Containers)
	assert.NotNil(t, agentContainer)
	assertVolumeMountPath(t, agentContainer.VolumeMounts, "data-volume", "/some/path/db")

	mongoContainer := container.GetByName("mongod", sts.Spec.Template.Spec.Containers)
	assert.NotNil(t, mongoContainer)

	lastCommand := mongoContainer.Command[len(agentContainer.Command)-1]
	assert.Contains(t, lastCommand, "/some/path/db", "startup command should be using the newly specified path")

	ac, err := automationconfig.ReadFromSecret(ctx, c, types.NamespacedName{Name: "example-mongodb-config", Namespace: "test-ns"})
	assert.NoError(t, err)

	for _, p := range ac.Processes {
		actualStoragePath := p.Args26.Get("storage.dbPath").String()
		assert.Equal(t, "/some/path/db", actualStoragePath, "process dbPath should have been set")
	}
}

func TestInconsistentReplicas(t *testing.T) {
	ctx := context.Background()
	mdb := newTestReplicaSet()
	stsReplicas := new(int32)
	*stsReplicas = 3
	mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.Replicas = stsReplicas
	mdb.Spec.Members = 4

	mgr := client.NewManager(ctx, &mdb)
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assert.NoError(t, err)
}

func assertVolumeMountPath(t *testing.T, mounts []corev1.VolumeMount, name, path string) {
	for _, v := range mounts {
		if v.Name == name {
			assert.Equal(t, path, v.MountPath)
			return
		}
	}
	t.Fatalf("volume with name %s was not present!", name)
}

func performReconciliationAndGetStatefulSet(ctx context.Context, t *testing.T, filePath string) (appsv1.StatefulSet, client.Client) {
	mdb, err := loadTestFixture(filePath)
	assert.NoError(t, err)
	mgr := client.NewManager(ctx, &mdb)
	assert.NoError(t, generatePasswordsForAllUsers(ctx, mdb, mgr.Client))
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)

	sts, err := mgr.Client.GetStatefulSet(ctx, mdb.NamespacedName())
	assert.NoError(t, err)
	return sts, mgr.Client
}

func performReconciliationAndGetService(ctx context.Context, t *testing.T, filePath string) (corev1.Service, client.Client) {
	mdb, err := loadTestFixture(filePath)
	assert.NoError(t, err)
	mgr := client.NewManager(ctx, &mdb)
	assert.NoError(t, generatePasswordsForAllUsers(ctx, mdb, mgr.Client))
	r := NewReconciler(mgr, "fake-mongodbRepoUrl", "fake-mongodbImage", "ubi8", AgentImage, "fake-versionUpgradeHookImage", "fake-readinessProbeImage")
	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)
	svc, err := mgr.Client.GetService(ctx, types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	return svc, mgr.Client
}

func generatePasswordsForAllUsers(ctx context.Context, mdb mdbv1.MongoDBCommunity, c client.Client) error {
	for _, user := range mdb.Spec.Users {

		key := "password"
		if user.PasswordSecretRef.Key != "" {
			key = user.PasswordSecretRef.Key
		}

		passwordSecret := secret.Builder().
			SetName(user.PasswordSecretRef.Name).
			SetNamespace(mdb.Namespace).
			SetField(key, "GAGTQK2ccRRaxJFudI5y").
			Build()

		if err := c.CreateSecret(ctx, passwordSecret); err != nil {
			return err
		}
	}

	return nil
}

func assertReconciliationSuccessful(t *testing.T, result reconcile.Result, err error) {
	assert.NoError(t, err)
	assert.Equal(t, false, result.Requeue)
	assert.Equal(t, time.Duration(0), result.RequeueAfter)
}

// makeStatefulSetReady updates the StatefulSet corresponding to the
// provided MongoDB resource to mark it as ready for the case of `statefulset.IsReady`
func makeStatefulSetReady(ctx context.Context, t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity) {
	setStatefulSetReadyReplicas(ctx, t, c, mdb, mdb.StatefulSetReplicasThisReconciliation())
}

func setStatefulSetReadyReplicas(ctx context.Context, t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, readyReplicas int) {
	sts := appsv1.StatefulSet{}
	err := c.Get(ctx, mdb.NamespacedName(), &sts)
	assert.NoError(t, err)
	sts.Status.ReadyReplicas = int32(readyReplicas)
	sts.Status.UpdatedReplicas = int32(mdb.StatefulSetReplicasThisReconciliation())
	err = c.Update(ctx, &sts)
	assert.NoError(t, err)
}

func setArbiterStatefulSetReadyReplicas(ctx context.Context, t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, readyReplicas int) {
	sts := appsv1.StatefulSet{}
	err := c.Get(ctx, mdb.ArbiterNamespacedName(), &sts)
	assert.NoError(t, err)
	sts.Status.ReadyReplicas = int32(readyReplicas)
	sts.Status.UpdatedReplicas = int32(mdb.StatefulSetArbitersThisReconciliation())
	err = c.Update(ctx, &sts)
	assert.NoError(t, err)
}

// loadTestFixture will create a MongoDB resource from a given fixture
func loadTestFixture(yamlFileName string) (mdbv1.MongoDBCommunity, error) {
	testPath := fmt.Sprintf("testdata/%s", yamlFileName)
	mdb := mdbv1.MongoDBCommunity{}
	data, err := os.ReadFile(testPath)
	if err != nil {
		return mdb, fmt.Errorf("error reading file: %s", err)
	}

	if err := marshalRuntimeObjectFromYAMLBytes(data, &mdb); err != nil {
		return mdb, fmt.Errorf("error converting yaml bytes to service account: %s", err)
	}

	return mdb, nil
}

// marshalRuntimeObjectFromYAMLBytes accepts the bytes of a yaml resource
// and unmarshals them into the provided runtime Object
func marshalRuntimeObjectFromYAMLBytes(bytes []byte, obj runtime.Object) error {
	jsonBytes, err := yaml.YAMLToJSON(bytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, &obj)
}

func TestGetMongoDBImage(t *testing.T) {
	type testConfig struct {
		mongodbRepoUrl   string
		mongodbImage     string
		mongodbImageType string
		version          string
		expectedImage    string
	}
	tests := map[string]testConfig{
		"Default UBI8 Community image": {
			mongodbRepoUrl:   "docker.io/mongodb",
			mongodbImage:     "mongodb-community-server",
			mongodbImageType: "ubi8",
			version:          "6.0.5",
			expectedImage:    "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8",
		},
		"Overridden UBI8 Enterprise image": {
			mongodbRepoUrl:   "docker.io/mongodb",
			mongodbImage:     "mongodb-enterprise-server",
			mongodbImageType: "ubi8",
			version:          "6.0.5",
			expectedImage:    "docker.io/mongodb/mongodb-enterprise-server:6.0.5-ubi8",
		},
		"Overridden UBI8 Enterprise image from Quay": {
			mongodbRepoUrl:   "quay.io/mongodb",
			mongodbImage:     "mongodb-enterprise-server",
			mongodbImageType: "ubi8",
			version:          "6.0.5",
			expectedImage:    "quay.io/mongodb/mongodb-enterprise-server:6.0.5-ubi8",
		},
		"Overridden Ubuntu Community image": {
			mongodbRepoUrl:   "docker.io/mongodb",
			mongodbImage:     "mongodb-community-server",
			mongodbImageType: "ubuntu2204",
			version:          "6.0.5",
			expectedImage:    "docker.io/mongodb/mongodb-community-server:6.0.5-ubuntu2204",
		},
		"Overridden UBI Community image": {
			mongodbRepoUrl:   "docker.io/mongodb",
			mongodbImage:     "mongodb-community-server",
			mongodbImageType: "ubi8",
			version:          "6.0.5",
			expectedImage:    "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8",
		},
		"Docker Inc images": {
			mongodbRepoUrl:   "docker.io",
			mongodbImage:     "mongo",
			mongodbImageType: "ubi8",
			version:          "6.0.5",
			expectedImage:    "docker.io/mongo:6.0.5",
		},
		"Deprecated AppDB images defined the old way": {
			mongodbRepoUrl: "quay.io",
			mongodbImage:   "mongodb/mongodb-enterprise-appdb-database-ubi",
			// In this example, we intentionally don't use the suffix from the env. variable and let users
			// define it in the version instead. There are some known customers who do this.
			// This is a backwards compatibility case.
			mongodbImageType: "will-be-ignored",
			version:          "5.0.14-ent",
			expectedImage:    "quay.io/mongodb/mongodb-enterprise-appdb-database-ubi:5.0.14-ent",
		},
	}
	for testName := range tests {
		t.Run(testName, func(t *testing.T) {
			testConfig := tests[testName]
			image := getMongoDBImage(testConfig.mongodbRepoUrl, testConfig.mongodbImage, testConfig.mongodbImageType, testConfig.version)
			assert.Equal(t, testConfig.expectedImage, image)
		})
	}
}
