package mongodbtests

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/wait"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// SkipTestIfLocal skips tests locally which tests connectivity to mongodb pods
func SkipTestIfLocal(t *testing.T, msg string, f func(t *testing.T)) {
	if testing.Short() {
		t.Log("Skipping [" + msg + "]")
		return
	}
	t.Run(msg, f)
}

// StatefulSetBecomesReady ensures that the underlying stateful set
// reaches the running state.
func StatefulSetBecomesReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	defaultOpts := []wait.Configuration{
		wait.RetryInterval(time.Second * 15),
		wait.Timeout(time.Minute * 25),
	}
	defaultOpts = append(defaultOpts, opts...)
	return statefulSetIsReady(ctx, mdb, defaultOpts...)
}

// ArbitersStatefulSetBecomesReady ensures that the underlying stateful set
// reaches the running state.
func ArbitersStatefulSetBecomesReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	defaultOpts := []wait.Configuration{
		wait.RetryInterval(time.Second * 15),
		wait.Timeout(time.Minute * 20),
	}
	defaultOpts = append(defaultOpts, opts...)
	return arbitersStatefulSetIsReady(ctx, mdb, defaultOpts...)
}

// StatefulSetBecomesUnready ensures the underlying stateful set reaches
// the unready state.
func StatefulSetBecomesUnready(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	defaultOpts := []wait.Configuration{
		wait.RetryInterval(time.Second * 15),
		wait.Timeout(time.Minute * 15),
	}
	defaultOpts = append(defaultOpts, opts...)
	return statefulSetIsNotReady(ctx, mdb, defaultOpts...)
}

// StatefulSetIsReadyAfterScaleDown ensures that a replica set is scaled down correctly
// note: scaling down takes considerably longer than scaling up due the readiness probe
// failure threshold being high
func StatefulSetIsReadyAfterScaleDown(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForStatefulSetToBeReadyAfterScaleDown(ctx, t, mdb, wait.RetryInterval(time.Second*60), wait.Timeout(time.Minute*45))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// statefulSetIsReady ensures that the underlying stateful set
// reaches the running state.
func statefulSetIsReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	return func(t *testing.T) {
		start := time.Now()
		err := wait.ForStatefulSetToBeReady(ctx, t, mdb, opts...)
		if err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start).Seconds()
		t.Logf("StatefulSet %s/%s is ready! It took %f seconds", mdb.Namespace, mdb.Name, elapsed)
	}
}

// arbitersStatefulSetIsReady ensures that the underlying stateful set
// reaches the running state.
func arbitersStatefulSetIsReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForArbitersStatefulSetToBeReady(ctx, t, mdb, opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Arbiters StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// statefulSetIsNotReady ensures that the underlying stateful set reaches the unready state.
func statefulSetIsNotReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...wait.Configuration) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForStatefulSetToBeUnready(ctx, t, mdb, opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is not ready!", mdb.Namespace, mdb.Name)
	}
}

func StatefulSetHasOwnerReference(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	return func(t *testing.T) {
		stsNamespacedName := types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}
		sts := appsv1.StatefulSet{}
		err := e2eutil.TestClient.Get(ctx, stsNamespacedName, &sts)

		if err != nil {
			t.Fatal(err)
		}
		assertEqualOwnerReference(t, "StatefulSet", stsNamespacedName, sts.GetOwnerReferences(), expectedOwnerReference)
	}
}

// StatefulSetIsDeleted ensures that the underlying stateful set is deleted
func StatefulSetIsDeleted(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForStatefulSetToBeDeleted(ctx, mdb.Name, time.Second*10, time.Minute*1, mdb.Namespace)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ServiceHasOwnerReference(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	return func(t *testing.T) {
		serviceNamespacedName := types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}
		srv := corev1.Service{}
		err := e2eutil.TestClient.Get(ctx, serviceNamespacedName, &srv)
		if err != nil {
			t.Fatal(err)
		}
		assertEqualOwnerReference(t, "Service", serviceNamespacedName, srv.GetOwnerReferences(), expectedOwnerReference)
	}
}

func ServiceUsesCorrectPort(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedPort int32) func(t *testing.T) {
	return func(t *testing.T) {
		serviceNamespacedName := types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace}
		svc := corev1.Service{}
		err := e2eutil.TestClient.Get(ctx, serviceNamespacedName, &svc)
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, svc.Spec.Ports, 1)
		assert.Equal(t, svc.Spec.Ports[0].Port, expectedPort)
	}
}

func AgentX509SecretsExists(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		agentCertSecret := corev1.Secret{}
		err := e2eutil.TestClient.Get(ctx, mdb.AgentCertificateSecretNamespacedName(), &agentCertSecret)
		assert.NoError(t, err)

		agentCertPemSecret := corev1.Secret{}
		err = e2eutil.TestClient.Get(ctx, mdb.AgentCertificatePemSecretNamespacedName(), &agentCertPemSecret)
		assert.NoError(t, err)
	}
}

func AgentSecretsHaveOwnerReference(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	checkSecret := func(t *testing.T, resourceNamespacedName types.NamespacedName) {
		secret := corev1.Secret{}
		err := e2eutil.TestClient.Get(ctx, resourceNamespacedName, &secret)

		assert.NoError(t, err)
		assertEqualOwnerReference(t, "Secret", resourceNamespacedName, secret.GetOwnerReferences(), expectedOwnerReference)
	}

	return func(t *testing.T) {
		checkSecret(t, mdb.GetAgentPasswordSecretNamespacedName())
		checkSecret(t, mdb.GetAgentKeyfileSecretNamespacedName())
	}
}

// ConnectionStringSecretsAreConfigured verifies that secrets storing the connection string were generated for all scram users
// and that they have the expected owner reference
func ConnectionStringSecretsAreConfigured(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	return func(t *testing.T) {
		for _, user := range mdb.GetAuthUsers() {
			secret := corev1.Secret{}
			secretNamespacedName := types.NamespacedName{Name: user.ConnectionStringSecretName, Namespace: mdb.Namespace}
			err := e2eutil.TestClient.Get(ctx, secretNamespacedName, &secret)

			assert.NoError(t, err)
			assertEqualOwnerReference(t, "Secret", secretNamespacedName, secret.GetOwnerReferences(), expectedOwnerReference)
		}
	}
}

// StatefulSetHasUpdateStrategy verifies that the StatefulSet holding this MongoDB
// resource has the correct Update Strategy
func StatefulSetHasUpdateStrategy(ctx context.Context, mdb *mdbv1.MongoDBCommunity, strategy appsv1.StatefulSetUpdateStrategyType) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForStatefulSetToHaveUpdateStrategy(ctx, t, mdb, strategy, wait.RetryInterval(time.Second*15), wait.Timeout(time.Minute*8))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// GetPersistentVolumes returns all persistent volumes on the cluster
func getPersistentVolumesList(ctx context.Context) (*corev1.PersistentVolumeList, error) {
	return e2eutil.TestClient.CoreV1Client.PersistentVolumes().List(ctx, metav1.ListOptions{})
}

func containsVolume(volumes []corev1.PersistentVolume, volumeName string) bool {
	for _, v := range volumes {
		if v.Name == volumeName {
			return true
		}
	}
	return false
}

func HasExpectedPersistentVolumes(ctx context.Context, volumes []corev1.PersistentVolume) func(t *testing.T) {
	return func(t *testing.T) {
		volumeList, err := getPersistentVolumesList(ctx)
		actualVolumes := volumeList.Items
		assert.NoError(t, err)
		assert.Len(t, actualVolumes, len(volumes),
			"The number of persistent volumes should be equal to the amount of volumes we created. Expected: %d, actual: %d",
			len(volumes), len(actualVolumes))
		for _, v := range volumes {
			assert.True(t, containsVolume(actualVolumes, v.Name))
		}
	}
}
func HasExpectedMetadata(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedLabels map[string]string, expectedAnnotations map[string]string) func(t *testing.T) {
	return func(t *testing.T) {
		namespace := mdb.Namespace

		statefulSetList := appsv1.StatefulSetList{}
		err := e2eutil.TestClient.Client.List(ctx, &statefulSetList, client.InNamespace(namespace))
		assert.NoError(t, err)
		assert.NotEmpty(t, statefulSetList.Items)
		for _, s := range statefulSetList.Items {
			containsMetadata(t, s.ObjectMeta, expectedLabels, expectedAnnotations, "statefulset "+s.Name)
		}

		volumeList := corev1.PersistentVolumeList{}
		err = e2eutil.TestClient.Client.List(ctx, &volumeList, client.InNamespace(namespace))
		assert.NoError(t, err)
		assert.NotEmpty(t, volumeList.Items)
		for _, s := range volumeList.Items {
			volName := s.Name
			if strings.HasPrefix(volName, "data-volume-") || strings.HasPrefix(volName, "logs-volume-") {
				containsMetadata(t, s.ObjectMeta, expectedLabels, expectedAnnotations, "volume "+volName)
			}
		}

		podList := corev1.PodList{}
		err = e2eutil.TestClient.Client.List(ctx, &podList, client.InNamespace(namespace))
		assert.NoError(t, err)
		assert.NotEmpty(t, podList.Items)

		for _, s := range podList.Items {
			// only consider stateful-sets (as opposite to the controller replica set)
			for _, owner := range s.OwnerReferences {
				if owner.Kind == "ReplicaSet" {
					continue
				}
			}
			// Ignore non-owned pods
			if len(s.OwnerReferences) == 0 {
				continue
			}

			// Ensure we are considering pods owned by a stateful set
			hasStatefulSetOwner := false
			for _, owner := range s.OwnerReferences {
				if owner.Kind == "StatefulSet" {
					hasStatefulSetOwner = true
				}
			}
			if !hasStatefulSetOwner {
				continue
			}

			containsMetadata(t, s.ObjectMeta, expectedLabels, expectedAnnotations, "pod "+s.Name)
		}
	}
}

func containsMetadata(t *testing.T, metadata metav1.ObjectMeta, expectedLabels map[string]string, expectedAnnotations map[string]string, msg string) {
	labels := metadata.Labels
	for k, v := range expectedLabels {
		assert.Contains(t, labels, k, msg+" has label "+k)
		value := labels[k]
		assert.Equal(t, v, value, msg+" has label "+k+" with value "+v)
	}

	annotations := metadata.Annotations
	for k, v := range expectedAnnotations {
		assert.Contains(t, annotations, k, msg+" has annotation "+k)
		value := annotations[k]
		assert.Equal(t, v, value, msg+" has annotation "+k+" with value "+v)
	}
}

// MongoDBReachesPendingPhase ensures the MongoDB resources gets to the Pending phase
func MongoDBReachesPendingPhase(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForMongoDBToReachPhase(ctx, t, mdb, mdbv1.Pending, time.Second*15, time.Minute*2)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is Pending!", mdb.Namespace, mdb.Name)
	}
}

// MongoDBReachesRunningPhase ensure the MongoDB resource reaches the Running phase
func MongoDBReachesRunningPhase(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForMongoDBToReachPhase(ctx, t, mdb, mdbv1.Running, time.Second*15, time.Minute*12)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is Running!", mdb.Namespace, mdb.Name)
	}
}

// MongoDBReachesFailedPhase ensure the MongoDB resource reaches the Failed phase.
func MongoDBReachesFailedPhase(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForMongoDBToReachPhase(ctx, t, mdb, mdbv1.Failed, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is in Failed state!", mdb.Namespace, mdb.Name)
	}
}

func AutomationConfigSecretExists(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		s, err := wait.ForSecretToExist(ctx, mdb.AutomationConfigSecretName(), time.Second*5, time.Minute*1, mdb.Namespace)
		assert.NoError(t, err)

		t.Logf("Secret %s/%s was successfully created", mdb.Namespace, mdb.AutomationConfigSecretName())
		assert.Contains(t, s.Data, automationconfig.ConfigKey)

		t.Log("The Secret contained the automation config")
	}
}

func getAutomationConfig(ctx context.Context, t *testing.T, mdb *mdbv1.MongoDBCommunity) automationconfig.AutomationConfig {
	currentSecret := corev1.Secret{}
	currentAc := automationconfig.AutomationConfig{}
	err := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace}, &currentSecret)
	assert.NoError(t, err)
	err = json.Unmarshal(currentSecret.Data[automationconfig.ConfigKey], &currentAc)
	assert.NoError(t, err)
	return currentAc
}

// AutomationConfigVersionHasTheExpectedVersion verifies that the automation config has the expected version.
func AutomationConfigVersionHasTheExpectedVersion(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedVersion int) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		assert.Equal(t, expectedVersion, currentAc.Version)
	}
}

// AutomationConfigHasLogRotationConfig verifies that the automation config contains the given logRotate config.
func AutomationConfigHasLogRotationConfig(ctx context.Context, mdb *mdbv1.MongoDBCommunity, lrc *automationconfig.CrdLogRotate) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		for _, p := range currentAc.Processes {
			assert.Equal(t, automationconfig.ConvertCrdLogRotateToAC(lrc), p.LogRotate)
		}
	}
}

func AutomationConfigHasSettings(ctx context.Context, mdb *mdbv1.MongoDBCommunity, settings map[string]interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		assert.Equal(t, currentAc.ReplicaSets[0].Settings, settings)
	}
}

// AutomationConfigReplicaSetsHaveExpectedArbiters verifies that the automation config has the expected version.
func AutomationConfigReplicaSetsHaveExpectedArbiters(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedArbiters int) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		lsRs := currentAc.ReplicaSets
		for _, rs := range lsRs {
			arbiters := 0
			for _, rsMember := range rs.Members {
				if rsMember.ArbiterOnly {
					arbiters += 1
				}
			}
			assert.Equal(t, expectedArbiters, arbiters)
		}
	}
}

// AutomationConfigHasTheExpectedCustomRoles verifies that the automation config has the expected custom roles.
func AutomationConfigHasTheExpectedCustomRoles(ctx context.Context, mdb *mdbv1.MongoDBCommunity, roles []automationconfig.CustomRole) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		assert.ElementsMatch(t, roles, currentAc.Roles)
	}
}

func AutomationConfigHasVoteTagPriorityConfigured(ctx context.Context, mdb *mdbv1.MongoDBCommunity, memberOptions []automationconfig.MemberOptions) func(t *testing.T) {
	acMemberOptions := make([]automationconfig.MemberOptions, 0)

	return func(t *testing.T) {
		currentAc := getAutomationConfig(ctx, t, mdb)
		rsMembers := currentAc.ReplicaSets
		sort.Slice(rsMembers[0].Members, func(i, j int) bool {
			return rsMembers[0].Members[i].Id < rsMembers[0].Members[j].Id
		})

		for _, m := range rsMembers[0].Members {
			acMemberOptions = append(acMemberOptions, automationconfig.MemberOptions{Votes: m.Votes, Priority: floatPtrTostringPtr(m.Priority), Tags: m.Tags})
		}
		assert.ElementsMatch(t, memberOptions, acMemberOptions)
	}
}

// CreateMongoDBResource creates the MongoDB resource
func CreateMongoDBResource(mdb *mdbv1.MongoDBCommunity, textCtx *e2eutil.TestContext) func(*testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.TestClient.Create(textCtx.Ctx, mdb, &e2eutil.CleanupOptions{TestContext: textCtx}); err != nil {
			t.Fatal(err)
		}
		t.Logf("Created MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
	}
}

// DeleteMongoDBResource deletes the MongoDB resource
func DeleteMongoDBResource(mdb *mdbv1.MongoDBCommunity, testCtx *e2eutil.TestContext) func(*testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.TestClient.Delete(testCtx.Ctx, mdb); err != nil {
			t.Fatal(err)
		}
		t.Logf("Deleted MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
	}
}

// GetConnectionStringSecret returnes the secret generated by the operator that is storing the connection string for a specific user
func GetConnectionStringSecret(ctx context.Context, mdb mdbv1.MongoDBCommunity, user authtypes.User) corev1.Secret {
	secret := corev1.Secret{}
	secretNamespacedName := types.NamespacedName{Name: user.ConnectionStringSecretName, Namespace: mdb.Namespace}
	_ = e2eutil.TestClient.Get(ctx, secretNamespacedName, &secret)
	return secret
}

// GetConnectionStringForUser returns the mongodb standard connection string for a user
func GetConnectionStringForUser(ctx context.Context, mdb mdbv1.MongoDBCommunity, user authtypes.User) string {
	return string(GetConnectionStringSecret(ctx, mdb, user).Data["connectionString.standard"])
}

// GetSrvConnectionStringForUser returns the mongodb service connection string for a user
func GetSrvConnectionStringForUser(ctx context.Context, mdb mdbv1.MongoDBCommunity, user authtypes.User) string {
	return string(GetConnectionStringSecret(ctx, mdb, user).Data["connectionString.standardSrv"])
}

func getOwnerReference(mdb *mdbv1.MongoDBCommunity) metav1.OwnerReference {
	return *metav1.NewControllerRef(mdb, schema.GroupVersionKind{
		Group:   mdbv1.GroupVersion.Group,
		Version: mdbv1.GroupVersion.Version,
		Kind:    mdb.Kind,
	})
}

func BasicFunctionality(ctx context.Context, mdb *mdbv1.MongoDBCommunity, skipStatusCheck ...bool) func(*testing.T) {
	return func(t *testing.T) {
		mdbOwnerReference := getOwnerReference(mdb)
		t.Run("Secret Was Correctly Created", AutomationConfigSecretExists(ctx, mdb))
		t.Run("Stateful Set Reaches Ready State", StatefulSetBecomesReady(ctx, mdb))
		t.Run("MongoDB Reaches Running Phase", MongoDBReachesRunningPhase(ctx, mdb))
		t.Run("Stateful Set Has OwnerReference", StatefulSetHasOwnerReference(ctx, mdb, mdbOwnerReference))
		t.Run("Service Set Has OwnerReference", ServiceHasOwnerReference(ctx, mdb, mdbOwnerReference))
		t.Run("Agent Secrets Have OwnerReference", AgentSecretsHaveOwnerReference(ctx, mdb, mdbOwnerReference))
		t.Run("Connection string secrets are configured", ConnectionStringSecretsAreConfigured(ctx, mdb, mdbOwnerReference))
		// TODO: this is temporary, remove the need for skipStatuscheck after 0.7.4 operator release
		if len(skipStatusCheck) > 0 && !skipStatusCheck[0] {
			t.Run("Test Status Was Updated", Status(ctx, mdb, mdbv1.MongoDBCommunityStatus{
				MongoURI:                   mdb.MongoURI(""),
				Phase:                      mdbv1.Running,
				Version:                    mdb.GetMongoDBVersion(),
				CurrentMongoDBMembers:      mdb.Spec.Members,
				CurrentStatefulSetReplicas: mdb.Spec.Members,
			}))
		}
	}
}

func BasicFunctionalityX509(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		mdbOwnerReference := getOwnerReference(mdb)
		t.Run("Secret Was Correctly Created", AutomationConfigSecretExists(ctx, mdb))
		t.Run("Stateful Set Reaches Ready State", StatefulSetBecomesReady(ctx, mdb))
		t.Run("MongoDB Reaches Running Phase", MongoDBReachesRunningPhase(ctx, mdb))
		t.Run("Stateful Set Has OwnerReference", StatefulSetHasOwnerReference(ctx, mdb, mdbOwnerReference))
		t.Run("Service Set Has OwnerReference", ServiceHasOwnerReference(ctx, mdb, mdbOwnerReference))
		t.Run("Connection string secrets are configured", ConnectionStringSecretsAreConfigured(ctx, mdb, mdbOwnerReference))
	}
}

// ServiceWithNameExists checks whether a service with the name serviceName exists
func ServiceWithNameExists(ctx context.Context, serviceName string, namespace string) func(t *testing.T) {
	return func(t *testing.T) {
		serviceNamespacedName := types.NamespacedName{Name: serviceName, Namespace: namespace}
		srv := corev1.Service{}
		err := e2eutil.TestClient.Get(ctx, serviceNamespacedName, &srv)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Service with name %s exists", serviceName)
	}
}

// DeletePod will delete a pod that belongs to this MongoDB resource's StatefulSet
func DeletePod(ctx context.Context, mdb *mdbv1.MongoDBCommunity, podNum int) func(*testing.T) {
	return func(t *testing.T) {
		pod := podFromMongoDBCommunity(mdb, podNum)
		if err := e2eutil.TestClient.Delete(ctx, &pod); err != nil {
			t.Fatal(err)
		}

		t.Logf("pod %s/%s deleted", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	}
}

// DeleteStatefulSet provides a wrapper to delete appsv1.StatefulSet types
func DeleteStatefulSet(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mdb.Name,
				Namespace: mdb.Namespace,
			},
		}
		if err := e2eutil.TestClient.Delete(ctx, &sts); err != nil {
			t.Fatal(err)
		}

		t.Logf("StatefulSet %s/%s deleted", sts.ObjectMeta.Namespace, sts.ObjectMeta.Name)
	}
}

// Status compares the given status to the actual status of the MongoDB resource
func Status(ctx context.Context, mdb *mdbv1.MongoDBCommunity, expectedStatus mdbv1.MongoDBCommunityStatus) func(t *testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, mdb); err != nil {
			t.Fatalf("error getting MongoDB resource: %s", err)
		}
		assert.Equal(t, expectedStatus, mdb.Status)
	}
}

// Scale update the MongoDB with a new number of members and updates the resource.
func Scale(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newMembers int) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Scaling Mongodb %s, to %d members", mdb.Name, newMembers)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Members = newMembers
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// ScaleArbiters update the MongoDB with a new number of arbiters and updates the resource.
func ScaleArbiters(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newArbiters int) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Scaling Mongodb %s, to %d members", mdb.Name, newArbiters)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Arbiters = newArbiters
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// DisableTLS changes the tls.enabled attribute to false.
func DisableTLS(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return tls(ctx, mdb, false)
}

// EnableTLS changes the tls.enabled attribute to true.
func EnableTLS(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return tls(ctx, mdb, true)
}

// tls function configures the security.tls.enabled attribute.
func tls(ctx context.Context, mdb *mdbv1.MongoDBCommunity, enabled bool) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Setting security.tls.enabled to %t", enabled)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Security.TLS.Enabled = enabled
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ChangeVersion(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newVersion string) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Changing versions from: %s to %s", mdb.Spec.Version, newVersion)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Version = newVersion
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ChangePort(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newPort int) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Changing port from: %d to %d", mdb.GetMongodConfiguration().GetDBPort(), newPort)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.AdditionalMongodConfig.SetDBPort(newPort)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func AddConnectionStringOption(ctx context.Context, mdb *mdbv1.MongoDBCommunity, key string, value interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		t.Logf("Adding %s:%v to connection string", key, value)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.AdditionalConnectionStringConfig.SetOption(key, value)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ResetConnectionStringOptions(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.AdditionalConnectionStringConfig = mdbv1.NewMapWrapper()
			db.Spec.Users[0].AdditionalConnectionStringConfig = mdbv1.NewMapWrapper()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func AddConnectionStringOptionToUser(ctx context.Context, mdb *mdbv1.MongoDBCommunity, key string, value interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		t.Logf("Adding %s:%v to connection string to first user", key, value)
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Users[0].AdditionalConnectionStringConfig.SetOption(key, value)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func StatefulSetContainerConditionIsTrue(ctx context.Context, mdb *mdbv1.MongoDBCommunity, containerName string, condition func(c corev1.Container) bool) func(*testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{}
		err := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
		if err != nil {
			t.Fatal(err)
		}

		existingContainer := container.GetByName(containerName, sts.Spec.Template.Spec.Containers)
		if existingContainer == nil {
			t.Fatalf(`No container found with name "%s" in StatefulSet pod template`, containerName)
		}

		if !condition(*existingContainer) {
			t.Fatalf(`Container "%s" does not satisfy condition`, containerName)
		}
	}
}

func StatefulSetConditionIsTrue(ctx context.Context, mdb *mdbv1.MongoDBCommunity, condition func(s appsv1.StatefulSet) bool) func(*testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{}
		err := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
		if err != nil {
			t.Fatal(err)
		}

		if !condition(sts) {
			t.Fatalf(`StatefulSet "%s" does not satisfy condition`, mdb.Name)
		}
	}
}

// PodContainerBecomesNotReady waits until the container with 'containerName' in the pod #podNum becomes not ready.
func PodContainerBecomesNotReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, podNum int, containerName string) func(*testing.T) {
	return func(t *testing.T) {
		pod := podFromMongoDBCommunity(mdb, podNum)
		assert.NoError(t, wait.ForPodReadiness(ctx, t, false, containerName, time.Minute*10, pod))
	}
}

// PodContainerBecomesReady waits until the container with 'containerName' in the pod #podNum becomes ready.
func PodContainerBecomesReady(ctx context.Context, mdb *mdbv1.MongoDBCommunity, podNum int, containerName string) func(*testing.T) {
	return func(t *testing.T) {
		pod := podFromMongoDBCommunity(mdb, podNum)
		assert.NoError(t, wait.ForPodReadiness(ctx, t, true, containerName, time.Minute*3, pod))
	}
}

func ExecInContainer(ctx context.Context, mdb *mdbv1.MongoDBCommunity, podNum int, containerName, command string) func(*testing.T) {
	return func(t *testing.T) {
		pod := podFromMongoDBCommunity(mdb, podNum)
		_, err := e2eutil.TestClient.Execute(ctx, pod, containerName, command)
		assert.NoError(t, err)
	}
}

// StatefulSetMessageIsReceived waits (up to 5 minutes) to get desiredMessageStatus as a mongodb message status or returns a fatal error.
func StatefulSetMessageIsReceived(mdb *mdbv1.MongoDBCommunity, testCtx *e2eutil.TestContext, desiredMessageStatus string) func(t *testing.T) {
	return func(t *testing.T) {
		err := wait.ForMongoDBMessageStatus(testCtx.Ctx, t, mdb, time.Second*15, time.Minute*5, desiredMessageStatus)
		if err != nil {
			t.Fatal(err)
		}

	}
}

func podFromMongoDBCommunity(mdb *mdbv1.MongoDBCommunity, podNum int) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", mdb.Name, podNum),
			Namespace: mdb.Namespace,
		},
	}
}

func assertEqualOwnerReference(t *testing.T, resourceType string, resourceNamespacedName types.NamespacedName, ownerReferences []metav1.OwnerReference, expectedOwnerReference metav1.OwnerReference) {
	assert.Len(t, ownerReferences, 1, fmt.Sprintf("%s %s/%s doesn't have OwnerReferences", resourceType, resourceNamespacedName.Name, resourceNamespacedName.Namespace))

	assert.Equal(t, expectedOwnerReference.APIVersion, ownerReferences[0].APIVersion)
	assert.Equal(t, "MongoDBCommunity", ownerReferences[0].Kind)
	assert.Equal(t, expectedOwnerReference.Name, ownerReferences[0].Name)
	assert.Equal(t, expectedOwnerReference.UID, ownerReferences[0].UID)
}

func RemoveLastUserFromMongoDBCommunity(ctx context.Context, mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Users = db.Spec.Users[:len(db.Spec.Users)-1]
		})

		if err != nil {
			t.Fatal(err)
		}
	}
}

func EditConnectionStringSecretNameOfLastUser(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newSecretName string) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Users[len(db.Spec.Users)-1].ConnectionStringSecretName = newSecretName
		})

		if err != nil {
			t.Fatal(err)
		}
	}
}

func ConnectionStringSecretIsCleanedUp(ctx context.Context, mdb *mdbv1.MongoDBCommunity, removedConnectionString string) func(t *testing.T) {
	return func(t *testing.T) {
		connectionStringSecret := corev1.Secret{}
		newErr := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: removedConnectionString, Namespace: mdb.Namespace}, &connectionStringSecret)

		assert.EqualError(t, newErr, fmt.Sprintf("secrets \"%s\" not found", removedConnectionString))
	}
}

func AuthUsersDeletedIsUpdated(ctx context.Context, mdb *mdbv1.MongoDBCommunity, mdbUser mdbv1.MongoDBUser) func(t *testing.T) {
	return func(t *testing.T) {
		deletedUser := automationconfig.DeletedUser{User: mdbUser.Name, Dbs: []string{mdbUser.DB}}

		currentAc := getAutomationConfig(ctx, t, mdb)

		assert.Contains(t, currentAc.Auth.UsersDeleted, deletedUser)
	}
}

func AddUserToMongoDBCommunity(ctx context.Context, mdb *mdbv1.MongoDBCommunity, newUser mdbv1.MongoDBUser) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(ctx, mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Users = append(db.Spec.Users, newUser)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func floatPtrTostringPtr(floatPtr *float32) *string {
	if floatPtr != nil {
		stringValue := fmt.Sprintf("%.1f", *floatPtr)
		return &stringValue
	}
	return nil
}
