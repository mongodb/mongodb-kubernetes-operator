package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/stretchr/testify/require"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/objx"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
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

func init() {
	os.Setenv(construct.AgentImageEnv, "agent-image")
}

func newTestReplicaSet() mdbv1.MongoDBCommunity {
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
	// TODO: Create builder/yaml fixture of some type to construct MDB objects for unit tests
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)

	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	s := corev1.Secret{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace}, &s)
	assert.NoError(t, err)
	assert.Equal(t, mdb.Namespace, s.Namespace)
	assert.Equal(t, mdb.AutomationConfigSecretName(), s.Name)
	assert.Contains(t, s.Data, automationconfig.ConfigKey)
	assert.NotEmpty(t, s.Data[automationconfig.ConfigKey])
}

func TestStatefulSet_IsCorrectlyConfigured(t *testing.T) {
	_ = os.Setenv(construct.MongodbRepoUrl, "repo")
	_ = os.Setenv(construct.MongodbImageEnv, "mongo")

	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	sts := appsv1.StatefulSet{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
	assert.NoError(t, err)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)

	agentContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, construct.AgentName, agentContainer.Name)
	assert.Equal(t, os.Getenv(construct.AgentImageEnv), agentContainer.Image)
	expectedProbe := probes.New(construct.DefaultReadiness())
	assert.True(t, reflect.DeepEqual(&expectedProbe, agentContainer.ReadinessProbe))

	mongodbContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, construct.MongodbName, mongodbContainer.Name)
	assert.Equal(t, "repo/mongo:4.2.2", mongodbContainer.Image)

	assert.Equal(t, resourcerequirements.Defaults(), agentContainer.Resources)

	acVolume, err := getVolumeByName(sts, "automation-config")
	assert.NoError(t, err)
	assert.NotNil(t, acVolume.Secret, "automation config should be stored in a secret!")
	assert.Nil(t, acVolume.ConfigMap, "automation config should be stored in a secret, not a config map!")

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
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	mgrClient := mgr.GetClient()
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})
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

	// reconcilliation is successful
	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
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
		mdb.Annotations[annotations.LastAppliedMongoDBVersion] = "4.0.0"
		sts, err := buildStatefulSet(mdb)
		assert.NoError(t, err)
		assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
	t.Run("On No Version Change, First Version", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Version = "4.0.0"
		delete(mdb.Annotations, annotations.LastAppliedMongoDBVersion)
		sts, err := buildStatefulSet(mdb)
		assert.NoError(t, err)
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
		sts, err := buildStatefulSet(mdb)

		assert.NoError(t, err)
		assert.Equal(t, appsv1.OnDeleteStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	})
}

func TestService_isCorrectlyCreatedAndUpdated(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 27017, Name: "mongodb"})

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func TestService_usesCustomMongodPortWhenSpecified(t *testing.T) {
	mdb := newTestReplicaSet()

	mongodConfig := objx.New(map[string]interface{}{})
	mongodConfig.Set("net.port", 1000.)
	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 1000, Name: "mongodb"})

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func createOrUpdatePodsWithVersions(t *testing.T, c k8sClient.Client, name types.NamespacedName, versions []string) {
	for i, version := range versions {
		createPodWithAgentAnnotation(t, c, types.NamespacedName{
			Namespace: name.Namespace,
			Name:      fmt.Sprintf("%s-%d", name.Name, i),
		}, version)
	}
}

func createPodWithAgentAnnotation(t *testing.T, c k8sClient.Client, name types.NamespacedName, versionStr string) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Annotations: map[string]string{
				"agent.mongodb.com/version": versionStr,
			},
		},
	}

	err := c.Create(context.TODO(), &pod)

	if err != nil && apiErrors.IsAlreadyExists(err) {
		err = c.Update(context.TODO(), &pod)
		assert.NoError(t, err)
	}

	assert.NoError(t, err)
}

func TestService_changesMongodPortOnRunningClusterWithArbiters(t *testing.T) {
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

	mgr := client.NewManager(&mdb)

	r := NewReconciler(mgr)

	t.Run("Prepare cluster with arbiters and change port", func(t *testing.T) {
		err := createUserPasswordSecret(mgr.Client, mdb, "password-secret-name", "pass")
		assert.NoError(t, err)

		mdb.Spec.Arbiters = 1
		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		assertReconciliationSuccessful(t, res, err)
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
		})
		_ = assertAutomationConfigVersion(t, mgr.Client, mdb, 1)

		setStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), namespacedName, []string{"1", "1", "1"})
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), arbiterNamespacedName, []string{"1"})

		res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		assertReconciliationSuccessful(t, res, err)
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
		})
		_ = assertAutomationConfigVersion(t, mgr.Client, mdb, 1)
		assertStatefulsetReady(t, mgr, namespacedName, 3)
		assertStatefulsetReady(t, mgr, arbiterNamespacedName, 1)

		mdb.Spec.AdditionalMongodConfig = mdbv1.NewMongodConfiguration()
		mdb.Spec.AdditionalMongodConfig.SetDBPort(newPort)

		err = mgr.GetClient().Update(context.TODO(), &mdb)
		assert.NoError(t, err)

		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Port should be changed only in the process #0", func(t *testing.T) {
		// port changes should be performed one at a time
		// should set port #0 to new one
		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)

		currentAc := assertAutomationConfigVersion(t, mgr.Client, mdb, 2)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in processes #0,#1", func(t *testing.T) {
		setStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), namespacedName, []string{"2", "2", "2"})
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), arbiterNamespacedName, []string{"2"})

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(t, mgr.Client, mdb, 3)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in processes #0,#1,#2", func(t *testing.T) {
		setStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), namespacedName, []string{"3", "3", "3"})
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), arbiterNamespacedName, []string{"3"})

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		require.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(t, mgr.Client, mdb, 4)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, oldPort, currentAc.Processes[3].GetPort())

		// not all ports are changed, so there are still two ports in the service
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("Ports should be changed in all processes", func(t *testing.T) {
		setStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), namespacedName, []string{"4", "4", "4"})
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), arbiterNamespacedName, []string{"4"})

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
		assert.NoError(t, err)
		assert.True(t, res.Requeue)
		currentAc := assertAutomationConfigVersion(t, mgr.Client, mdb, 5)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[3].GetPort())

		// all the ports are changed but there are still two service ports for old and new port until the next reconcile
		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			oldPort: "mongodb",
			newPort: "mongodb-new",
		})

		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, oldPort, newPort)
	})

	t.Run("At the end there should be only new port in the service", func(t *testing.T) {
		setStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 3)
		setArbiterStatefulSetReadyReplicas(t, mgr.GetClient(), mdb, 1)
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), namespacedName, []string{"5", "5", "5"})
		createOrUpdatePodsWithVersions(t, mgr.GetClient(), arbiterNamespacedName, []string{"5"})

		res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: namespacedName})
		assert.NoError(t, err)
		// no need to requeue, port change is finished
		assert.False(t, res.Requeue)
		// there should not be any changes in config anymore
		currentAc := assertAutomationConfigVersion(t, mgr.Client, mdb, 5)
		require.Len(t, currentAc.Processes, 4)
		assert.Equal(t, newPort, currentAc.Processes[0].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[1].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[2].GetPort())
		assert.Equal(t, newPort, currentAc.Processes[3].GetPort())

		assertServicePorts(t, mgr.Client, mdb, map[int]string{
			newPort: "mongodb",
		})

		// only at the end, when all pods are ready we have updated connection strings
		assertConnectionStringSecretPorts(t, mgr.GetClient(), mdb, newPort, oldPort)
	})
}

// assertConnectionStringSecretPorts checks that connection string secret has expectedPort and does not have notExpectedPort.
func assertConnectionStringSecretPorts(t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, expectedPort int, notExpectedPort int) {
	connectionStringSecret := corev1.Secret{}
	scramUsers := mdb.GetScramUsers()
	require.Len(t, scramUsers, 1)
	secretNamespacedName := types.NamespacedName{Name: scramUsers[0].ConnectionStringSecretName, Namespace: mdb.Namespace}
	err := c.Get(context.TODO(), secretNamespacedName, &connectionStringSecret)
	require.NoError(t, err)
	require.Contains(t, connectionStringSecret.Data, "connectionString.standard")
	assert.Contains(t, string(connectionStringSecret.Data["connectionString.standard"]), fmt.Sprintf("%d", expectedPort))
	assert.NotContains(t, string(connectionStringSecret.Data["connectionString.standard"]), fmt.Sprintf("%d", notExpectedPort))
}

func assertServicePorts(t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, expectedServicePorts map[int]string) {
	svc := corev1.Service{}

	err := c.Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
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

func assertAutomationConfigVersion(t *testing.T, c client.Client, mdb mdbv1.MongoDBCommunity, expectedVersion int) automationconfig.AutomationConfig {
	ac, err := automationconfig.ReadFromSecret(c, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, ac.Version)
	return ac
}

func assertStatefulsetReady(t *testing.T, mgr manager.Manager, name types.NamespacedName, expectedReplicas int) {
	sts := appsv1.StatefulSet{}
	err := mgr.GetClient().Get(context.TODO(), name, &sts)
	require.NoError(t, err)
	assert.True(t, statefulset.IsReady(sts, expectedReplicas))
}

func TestService_configuresPrometheusCustomPorts(t *testing.T) {
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

	mgr := client.NewManager(&mdb)
	err := secret.CreateOrUpdate(mgr.Client,
		secret.Builder().
			SetName("secret").
			SetNamespace(mdb.Namespace).
			SetField("password", "my-password").
			Build(),
	)

	assert.NoError(t, err)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceTypeClusterIP)
	assert.Equal(t, svc.Spec.Selector["app"], mdb.ServiceName())
	assert.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 1000, Name: "mongodb"})
	assert.Equal(t, svc.Spec.Ports[1], corev1.ServicePort{Port: 4321, Name: "prometheus"})

	assert.Equal(t, svc.Labels["app"], mdb.ServiceName())

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)
}

func TestService_configuresPrometheus(t *testing.T) {
	mdb := newTestReplicaSet()
	mdb.Spec.Prometheus = &mdbv1.Prometheus{
		Username: "username",
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Name: "secret",
		},
	}

	mgr := client.NewManager(&mdb)
	err := secret.CreateOrUpdate(mgr.Client,
		secret.Builder().
			SetName("secret").
			SetNamespace(mdb.Namespace).
			SetField("password", "my-password").
			Build(),
	)
	assert.NoError(t, err)

	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	svc := corev1.Service{}
	err = mgr.GetClient().Get(context.TODO(), types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}, &svc)
	assert.NoError(t, err)

	assert.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, svc.Spec.Ports[0], corev1.ServicePort{Port: 27017, Name: "mongodb"})
	assert.Equal(t, svc.Spec.Ports[1], corev1.ServicePort{Port: 9216, Name: "prometheus"})
}

func TestCustomNetPort_Configuration(t *testing.T) {
	svc, _ := performReconciliationAndGetService(t, "specify_net_port.yaml")
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, corev1.ServicePort{Port: 40333, Name: "mongodb"}, svc.Spec.Ports[0])
}

func TestAutomationConfig_versionIsBumpedOnChange(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, 1, currentAc.Version)

	mdb.Spec.Members++
	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	_ = mgr.GetClient().Update(context.TODO(), &mdb)
	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, 2, currentAc.Version)
}

func TestAutomationConfig_versionIsNotBumpedWithNoChanges(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Equal(t, currentAc.Version, 1)
}

func TestAutomationConfigFCVIsNotIncreasedWhenUpgradingMinorVersion(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Len(t, currentAc.Processes, 3)
	assert.Equal(t, currentAc.Processes[0].FeatureCompatibilityVersion, "4.2")

	// Upgrading minor version does not change the FCV on the automationConfig
	mdbRef := &mdb
	mdbRef.Spec.Version = "4.4.0"
	_ = mgr.Client.Update(context.TODO(), mdbRef)
	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err = automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.Len(t, currentAc.Processes, 3)
	assert.Equal(t, currentAc.Processes[0].FeatureCompatibilityVersion, "4.2")

}

func TestAutomationConfig_CustomMongodConfig(t *testing.T) {
	mdb := newTestReplicaSet()

	mongodConfig := objx.New(map[string]interface{}{})
	mongodConfig.Set("net.port", 1000)
	mongodConfig.Set("storage.other", "value")
	mongodConfig.Set("arbitrary.config.path", "value")
	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
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
	mdb := newScramReplicaSet()
	mgr := client.NewManager(&mdb)

	c := mgr.Client

	keyFileNsName := mdb.GetAgentKeyfileSecretNamespacedName()
	err := secret.CreateOrUpdate(c,
		secret.Builder().
			SetName(keyFileNsName.Name).
			SetNamespace(keyFileNsName.Namespace).
			SetField(scram.AgentKeyfileKey, "my-keyfile").
			Build(),
	)
	assert.NoError(t, err)

	passwordNsName := mdb.GetAgentPasswordSecretNamespacedName()
	err = secret.CreateOrUpdate(c,
		secret.Builder().
			SetName(passwordNsName.Name).
			SetNamespace(passwordNsName.Namespace).
			SetField(scram.AgentPasswordKey, "my-pass").
			Build(),
	)
	assert.NoError(t, err)

	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.False(t, currentAc.Auth.Disabled)

	assert.Equal(t, "my-keyfile", currentAc.Auth.Key)
	assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
	assert.Equal(t, "my-pass", currentAc.Auth.AutoPwd)

}

func TestScramIsConfigured(t *testing.T) {
	assertReplicaSetIsConfiguredWithScram(t, newScramReplicaSet())
}

func TestScramIsConfiguredWhenNotSpecified(t *testing.T) {
	assertReplicaSetIsConfiguredWithScram(t, newTestReplicaSet())
}

func TestReplicaSet_IsScaledDown_OneMember_AtATime_WhenItAlreadyExists(t *testing.T) {
	mdb := newTestReplicaSet()
	mdb.Spec.Members = 5

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)

	assert.NoError(t, err)
	assert.Equal(t, 5, mdb.Status.CurrentMongoDBMembers)

	// scale members from five to three
	mdb.Spec.Members = 3

	err = mgr.GetClient().Update(context.TODO(), &mdb)
	assert.NoError(t, err)

	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})

	makeStatefulSetReady(t, mgr.GetClient(), mdb)
	assert.NoError(t, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, true, res.Requeue)
	assert.Equal(t, 4, mdb.Status.CurrentMongoDBMembers)

	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)
	assert.Equal(t, false, res.Requeue)
	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)
}

func TestReplicaSet_IsScaledUp_OneMember_AtATime_WhenItAlreadyExists(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)
	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)

	// scale members from three to five
	mdb.Spec.Members = 5

	err = mgr.GetClient().Update(context.TODO(), &mdb)
	assert.NoError(t, err)

	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)

	assert.NoError(t, err)
	assert.Equal(t, true, res.Requeue)
	assert.Equal(t, 4, mdb.Status.CurrentMongoDBMembers)

	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	makeStatefulSetReady(t, mgr.GetClient(), mdb)

	res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})

	assert.NoError(t, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, false, res.Requeue)
	assert.Equal(t, 5, mdb.Status.CurrentMongoDBMembers)
}

func TestIgnoreUnknownUsers(t *testing.T) {
	t.Run("Ignore Unkown Users set to true", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ignoreUnknownUsers := true
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = &ignoreUnknownUsers

		assertAuthoritativeSet(t, mdb, false)
	})

	t.Run("IgnoreUnknownUsers is not set", func(t *testing.T) {
		mdb := newTestReplicaSet()
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = nil
		assertAuthoritativeSet(t, mdb, false)
	})

	t.Run("IgnoreUnknownUsers set to false", func(t *testing.T) {
		mdb := newTestReplicaSet()
		ignoreUnknownUsers := false
		mdb.Spec.Security.Authentication.IgnoreUnknownUsers = &ignoreUnknownUsers
		assertAuthoritativeSet(t, mdb, true)
	})
}

func TestAnnotationsAreAppliedToResource(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.NotNil(t, mdb.Annotations)
	assert.NotEmpty(t, mdb.Annotations[lastSuccessfulConfiguration], "last successful spec should have been saved as annotation but was not")
	assert.Equal(t, mdb.Annotations[lastAppliedMongoDBVersion], mdb.Spec.Version, "last version should have been saved as an annotation but was not")
}

// assertAuthoritativeSet asserts that a reconciliation of the given MongoDBCommunity resource
// results in the AuthoritativeSet of the created AutomationConfig to have the expectedValue provided.
func assertAuthoritativeSet(t *testing.T, mdb mdbv1.MongoDBCommunity, expectedValue bool) {
	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	s, err := mgr.Client.GetSecret(types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)

	bytes := s.Data[automationconfig.ConfigKey]
	ac, err := automationconfig.FromBytes(bytes)
	assert.NoError(t, err)

	assert.Equal(t, expectedValue, ac.Auth.AuthoritativeSet)
}

func assertReplicaSetIsConfiguredWithScram(t *testing.T, mdb mdbv1.MongoDBCommunity) {
	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	currentAc, err := automationconfig.ReadFromSecret(mgr.Client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	t.Run("Automation Config is configured with SCRAM", func(t *testing.T) {
		assert.NotEmpty(t, currentAc.Auth.Key)
		assert.NoError(t, err)
		assert.NotEmpty(t, currentAc.Auth.KeyFileWindows)
		assert.NotEmpty(t, currentAc.Auth.AutoPwd)
		assert.False(t, currentAc.Auth.Disabled)
	})
	t.Run("Secret with password was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentPasswordSecretNamespacedName()
		s, err := mgr.Client.GetSecret(secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[scram.AgentPasswordKey], []byte(currentAc.Auth.AutoPwd))
	})

	t.Run("Secret with keyfile was created", func(t *testing.T) {
		secretNsName := mdb.GetAgentKeyfileSecretNamespacedName()
		s, err := mgr.Client.GetSecret(secretNsName)
		assert.NoError(t, err)
		assert.Equal(t, s.Data[scram.AgentKeyfileKey], []byte(currentAc.Auth.Key))
	})
}

func TestReplicaSet_IsScaledUpToDesiredMembers_WhenFirstCreated(t *testing.T) {
	mdb := newTestReplicaSet()

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	assertReconciliationSuccessful(t, res, err)

	err = mgr.GetClient().Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, 3, mdb.Status.CurrentMongoDBMembers)
}

func TestVolumeClaimTemplates_Configuration(t *testing.T) {
	sts, _ := performReconciliationAndGetStatefulSet(t, "volume_claim_templates_mdb.yaml")

	assert.Len(t, sts.Spec.VolumeClaimTemplates, 3)

	pvcSpec := sts.Spec.VolumeClaimTemplates[2].Spec

	storage := pvcSpec.Resources.Requests[corev1.ResourceStorage]
	storageRef := &storage

	assert.Equal(t, "1Gi", storageRef.String())
	assert.Len(t, pvcSpec.AccessModes, 1)
	assert.Contains(t, pvcSpec.AccessModes, corev1.ReadWriteOnce)
}

func TestChangeDataVolume_Configuration(t *testing.T) {
	sts, _ := performReconciliationAndGetStatefulSet(t, "change_data_volume.yaml")
	assert.Len(t, sts.Spec.VolumeClaimTemplates, 2)

	dataVolume := sts.Spec.VolumeClaimTemplates[0]

	storage := dataVolume.Spec.Resources.Requests[corev1.ResourceStorage]
	storageRef := &storage

	assert.Equal(t, "data-volume", dataVolume.Name)
	assert.Equal(t, "50Gi", storageRef.String())
}

func TestCustomStorageClass_Configuration(t *testing.T) {
	sts, _ := performReconciliationAndGetStatefulSet(t, "custom_storage_class.yaml")

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
	sts, _ := performReconciliationAndGetStatefulSet(t, "tolerations_example.yaml")

	assert.Len(t, sts.Spec.Template.Spec.Tolerations, 2)
	assert.Equal(t, "example-key", sts.Spec.Template.Spec.Tolerations[0].Key)
	assert.Equal(t, corev1.TolerationOpExists, sts.Spec.Template.Spec.Tolerations[0].Operator)
	assert.Equal(t, corev1.TaintEffectNoSchedule, sts.Spec.Template.Spec.Tolerations[0].Effect)

	assert.Equal(t, "example-key-2", sts.Spec.Template.Spec.Tolerations[1].Key)
	assert.Equal(t, corev1.TolerationOpEqual, sts.Spec.Template.Spec.Tolerations[1].Operator)
	assert.Equal(t, corev1.TaintEffectNoExecute, sts.Spec.Template.Spec.Tolerations[1].Effect)
}

func TestCustomDataDir_Configuration(t *testing.T) {
	sts, c := performReconciliationAndGetStatefulSet(t, "specify_data_dir.yaml")

	agentContainer := container.GetByName("mongodb-agent", sts.Spec.Template.Spec.Containers)
	assert.NotNil(t, agentContainer)
	assertVolumeMountPath(t, agentContainer.VolumeMounts, "data-volume", "/some/path/db")

	mongoContainer := container.GetByName("mongod", sts.Spec.Template.Spec.Containers)
	assert.NotNil(t, mongoContainer)

	lastCommand := mongoContainer.Command[len(agentContainer.Command)-1]
	assert.Contains(t, lastCommand, "/some/path/db", "startup command should be using the newly specified path")

	ac, err := automationconfig.ReadFromSecret(c, types.NamespacedName{Name: "example-mongodb-config", Namespace: "test-ns"})
	assert.NoError(t, err)

	for _, p := range ac.Processes {
		actualStoragePath := p.Args26.Get("storage.dbPath").String()
		assert.Equal(t, "/some/path/db", actualStoragePath, "process dbPath should have been set")
	}
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

func performReconciliationAndGetStatefulSet(t *testing.T, filePath string) (appsv1.StatefulSet, client.Client) {
	mdb, err := loadTestFixture(filePath)
	assert.NoError(t, err)
	mgr := client.NewManager(&mdb)
	assert.NoError(t, generatePasswordsForAllUsers(mdb, mgr.Client))
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)

	sts, err := mgr.Client.GetStatefulSet(mdb.NamespacedName())
	assert.NoError(t, err)
	return sts, mgr.Client
}

func performReconciliationAndGetService(t *testing.T, filePath string) (corev1.Service, client.Client) {
	mdb, err := loadTestFixture(filePath)
	assert.NoError(t, err)
	mgr := client.NewManager(&mdb)
	assert.NoError(t, generatePasswordsForAllUsers(mdb, mgr.Client))
	r := NewReconciler(mgr)
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: mdb.NamespacedName()})
	assertReconciliationSuccessful(t, res, err)
	svc, err := mgr.Client.GetService(types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace})
	assert.NoError(t, err)
	return svc, mgr.Client
}

func generatePasswordsForAllUsers(mdb mdbv1.MongoDBCommunity, c client.Client) error {
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

		if err := c.CreateSecret(passwordSecret); err != nil {
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
func makeStatefulSetReady(t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity) {
	setStatefulSetReadyReplicas(t, c, mdb, mdb.StatefulSetReplicasThisReconciliation())
}

func setStatefulSetReadyReplicas(t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, readyReplicas int) {
	sts := appsv1.StatefulSet{}
	err := c.Get(context.TODO(), mdb.NamespacedName(), &sts)
	assert.NoError(t, err)
	sts.Status.ReadyReplicas = int32(readyReplicas)
	sts.Status.UpdatedReplicas = int32(mdb.StatefulSetReplicasThisReconciliation())
	err = c.Update(context.TODO(), &sts)
	assert.NoError(t, err)
}

func setArbiterStatefulSetReadyReplicas(t *testing.T, c k8sClient.Client, mdb mdbv1.MongoDBCommunity, readyReplicas int) {
	sts := appsv1.StatefulSet{}
	err := c.Get(context.TODO(), mdb.ArbiterNamespacedName(), &sts)
	assert.NoError(t, err)
	sts.Status.ReadyReplicas = int32(readyReplicas)
	sts.Status.UpdatedReplicas = int32(mdb.StatefulSetArbitersThisReconciliation())
	err = c.Update(context.TODO(), &sts)
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
