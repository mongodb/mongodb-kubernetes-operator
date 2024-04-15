package mongotester

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/objx"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Tester struct {
	ctx         context.Context
	mongoClient *mongo.Client
	clientOpts  []*options.ClientOptions
	resource    *mdbv1.MongoDBCommunity
}

func newTester(ctx context.Context, mdb *mdbv1.MongoDBCommunity, opts ...*options.ClientOptions) *Tester {
	t := &Tester{
		ctx:      ctx,
		resource: mdb,
	}
	t.clientOpts = append(t.clientOpts, opts...)
	return t
}

// OptionApplier is an interface which is able to accept a list
// of options.ClientOptions, and return the final desired list
// making any modifications required
type OptionApplier interface {
	ApplyOption(opts ...*options.ClientOptions) []*options.ClientOptions
}

// FromResource returns a Tester instance from a MongoDB resource. It infers SCRAM username/password
// and the hosts from the resource.
func FromResource(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity, opts ...OptionApplier) (*Tester, error) {
	var clientOpts []*options.ClientOptions

	clientOpts = WithHosts(mdb.Hosts("")).ApplyOption(clientOpts...)

	t.Logf("Configuring hosts: %s for MongoDB: %s", mdb.Hosts(""), mdb.NamespacedName())

	users := mdb.Spec.Users
	if len(users) == 1 {
		user := users[0]
		passwordSecret := corev1.Secret{}
		err := e2eutil.TestClient.Get(ctx, types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: mdb.Namespace}, &passwordSecret)
		if err != nil {
			return nil, err
		}
		t.Logf("Configuring SCRAM username: %s and password from secret %s for MongoDB: %s", user.Name, user.PasswordSecretRef.Name, mdb.NamespacedName())
		clientOpts = WithScram(user.Name, string(passwordSecret.Data[user.PasswordSecretRef.Key])).ApplyOption(clientOpts...)
	}

	// add any additional options
	for _, opt := range opts {
		clientOpts = opt.ApplyOption(clientOpts...)
	}

	return newTester(ctx, &mdb, clientOpts...), nil
}

func FromX509Resource(ctx context.Context, t *testing.T, mdb mdbv1.MongoDBCommunity, opts ...OptionApplier) (*Tester, error) {
	var clientOpts []*options.ClientOptions

	clientOpts = WithHosts(mdb.Hosts("")).ApplyOption(clientOpts...)

	t.Logf("Configuring hosts: %s for MongoDB: %s", mdb.Hosts(""), mdb.NamespacedName())

	users := mdb.Spec.Users
	if len(users) == 1 {
		clientOpts = WithX509().ApplyOption(clientOpts...)
	}

	// add any additional options
	for _, opt := range opts {
		clientOpts = opt.ApplyOption(clientOpts...)
	}

	return newTester(ctx, &mdb, clientOpts...), nil
}

// ConnectivitySucceeds performs a basic check that ensures that it is possible
// to connect to the MongoDB resource
func (m *Tester) ConnectivitySucceeds(opts ...OptionApplier) func(t *testing.T) {
	return m.connectivityCheck(true, opts...)
}

// ConnectivityFails performs a basic check that ensures that it is not possible
// to connect to the MongoDB resource
func (m *Tester) ConnectivityFails(opts ...OptionApplier) func(t *testing.T) {
	return m.connectivityCheck(false, opts...)
}

func (m *Tester) ConnectivityRejected(ctx context.Context, opts ...OptionApplier) func(t *testing.T) {
	clientOpts := make([]*options.ClientOptions, 0)
	for _, optApplier := range opts {
		clientOpts = optApplier.ApplyOption(clientOpts...)
	}

	return func(t *testing.T) {
		// We can optionally skip connectivity tests locally
		if testing.Short() {
			t.Skip()
		}

		if err := m.ensureClient(ctx, clientOpts...); err == nil {
			t.Fatalf("No error, but it should have failed")
		}
	}
}

func (m *Tester) HasKeyfileAuth(tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("clusterAuthMode", "keyFile", tries, opts...)
}

func (m *Tester) HasFCV(fcv string, tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("featureCompatibilityVersion", map[string]interface{}{"version": fcv}, tries, opts...)
}

func (m *Tester) ScramIsConfigured(tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("authenticationMechanisms", primitive.A{"SCRAM-SHA-256"}, tries, opts...)
}

func (m *Tester) ScramWithAuthIsConfigured(tries int, enabledMechanisms primitive.A, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("authenticationMechanisms", enabledMechanisms, tries, opts...)
}

func (m *Tester) EnsureAuthenticationIsConfigured(tries int, opts ...OptionApplier) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("Ensure keyFile authentication is configured", m.HasKeyfileAuth(tries, opts...))
		t.Run("SCRAM-SHA-256 is configured", m.ScramIsConfigured(tries, opts...))
	}
}

func (m *Tester) EnsureAuthenticationWithAuthIsConfigured(tries int, enabledMechanisms primitive.A, opts ...OptionApplier) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("Ensure keyFile authentication is configured", m.HasKeyfileAuth(tries, opts...))
		t.Run(fmt.Sprintf("%q is configured", enabledMechanisms), m.ScramWithAuthIsConfigured(tries, enabledMechanisms, opts...))
	}
}

func (m *Tester) HasTlsMode(tlsMode string, tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("sslMode", tlsMode, tries, opts...)
}

// CustomRolesResult is a type to decode the result of getting rolesInfo.
type CustomRolesResult struct {
	Roles []automationconfig.CustomRole
}

func (m *Tester) VerifyRoles(expectedRoles []automationconfig.CustomRole, tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminCommandResult(func(t *testing.T) bool {
		var result CustomRolesResult
		err := m.mongoClient.Database("admin").
			RunCommand(m.ctx,
				bson.D{
					{Key: "rolesInfo", Value: 1},
					{Key: "showPrivileges", Value: true},
					{Key: "showBuiltinRoles", Value: false},
				}).Decode(&result)
		if err != nil {
			t.Fatal(err)
			return false
		}
		assert.ElementsMatch(t, result.Roles, expectedRoles)
		return true
	}, tries, opts...)
}

type verifyAdminResultFunc func(t *testing.T) bool

func (m *Tester) hasAdminCommandResult(verify verifyAdminResultFunc, tries int, opts ...OptionApplier) func(t *testing.T) {
	clientOpts := make([]*options.ClientOptions, 0)
	for _, optApplier := range opts {
		clientOpts = optApplier.ApplyOption(clientOpts...)
	}

	return func(t *testing.T) {
		if err := m.ensureClient(m.ctx, clientOpts...); err != nil {
			t.Fatal(err)
		}

		database := m.mongoClient.Database("admin")
		assert.NotNil(t, database)

		found := false
		for !found && tries > 0 {
			<-time.After(10 * time.Second)
			found = verify(t)
			tries--
		}
		assert.True(t, found)
	}
}

func (m *Tester) hasAdminParameter(key string, expectedValue interface{}, tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminCommandResult(func(t *testing.T) bool {
		var result map[string]interface{}
		err := m.mongoClient.Database("admin").
			RunCommand(m.ctx, bson.D{{Key: "getParameter", Value: 1}, {Key: key, Value: 1}}).
			Decode(&result)
		if err != nil {
			t.Logf("Unable to get admin setting %s with error : %s", key, err)
			return false
		}

		actualValue := result[key]
		t.Logf("Actual Value: %+v, type: %s", actualValue, reflect.TypeOf(actualValue))
		return reflect.DeepEqual(expectedValue, actualValue)
	}, tries, opts...)
}

func (m *Tester) connectivityCheck(shouldSucceed bool, opts ...OptionApplier) func(t *testing.T) {

	clientOpts := make([]*options.ClientOptions, 0)
	for _, optApplier := range opts {
		clientOpts = optApplier.ApplyOption(clientOpts...)
	}

	connectivityOpts := defaults()
	return func(t *testing.T) {

		// We can optionally skip connectivity tests locally
		if testing.Short() {
			t.Skip()
		}

		ctx, cancel := context.WithTimeout(m.ctx, connectivityOpts.ContextTimeout)
		defer cancel()

		if err := m.ensureClient(ctx, clientOpts...); err != nil {
			t.Fatal(err)
		}

		attempts := 0
		// There can be a short time before the user can auth as the user
		err := wait.PollUntilContextTimeout(ctx, connectivityOpts.IntervalTime, connectivityOpts.TimeoutTime, false, func(ctx context.Context) (done bool, err error) {
			attempts++
			collection := m.mongoClient.Database(connectivityOpts.Database).Collection(connectivityOpts.Collection)
			_, err = collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
			if err != nil && shouldSucceed {
				t.Logf("Was not able to connect, when we should have been able to!")
				return false, nil
			}
			if err == nil && !shouldSucceed {
				t.Logf("Was successfully able to connect, when we should not have been able to!")
				return false, nil
			}
			// this information is only useful if we needed more than one attempt.
			if attempts >= 2 {
				t.Logf("Connectivity check was successful after %d attempt(s)", attempts)
			}
			return true, nil
		})

		if err != nil {
			t.Fatal(fmt.Errorf("error during connectivity check: %s", err))
		}
	}
}

func (m *Tester) WaitForRotatedCertificate(mdb mdbv1.MongoDBCommunity, initialCertSerialNumber *big.Int) func(*testing.T) {
	return func(t *testing.T) {
		tls, err := getClientTLSConfig(m.ctx, mdb)
		assert.NoError(t, err)

		// Reject all server certificates that don't have the expected serial number
		tls.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert := verifiedChains[0][0]
			if initialCertSerialNumber.Cmp(cert.SerialNumber) == 0 {
				return fmt.Errorf("certificate serial number has not changed: %s", cert.SerialNumber)
			}
			return nil
		}

		if err := m.ensureClient(m.ctx, &options.ClientOptions{TLSConfig: tls}); err != nil {
			t.Fatal(err)
		}

		// Ping the cluster until it succeeds. The ping will only succeed with the right certificate.
		err = wait.PollUntilContextTimeout(m.ctx, 5*time.Second, 5*time.Minute, false, func(ctx context.Context) (done bool, err error) {
			if err := m.mongoClient.Ping(m.ctx, nil); err != nil {
				return false, nil
			}
			return true, nil
		})
		assert.NoError(t, err)
	}
}

// EnsureMongodConfig is mostly used for checking port changes. Port changes take some until they finish.
// We cannot fully rely on the statefulset or resource being ready/running since it will change its state multiple
// times during a port change. That means a resource might leave, go into and leave running multiple times until
// it truly finished its port change.
func (m *Tester) EnsureMongodConfig(selector string, expected interface{}) func(*testing.T) {
	return func(t *testing.T) {
		connectivityOpts := defaults()
		err := wait.PollUntilContextTimeout(m.ctx, connectivityOpts.IntervalTime, connectivityOpts.TimeoutTime, false, func(ctx context.Context) (done bool, err error) {
			opts, err := m.getCommandLineOptions()
			assert.NoError(t, err)

			parsed := objx.New(bsonToMap(opts)).Get("parsed").ObjxMap()

			return expected == parsed.Get(selector).Data(), nil
		})

		assert.NoError(t, err)

	}
}

// getCommandLineOptions will get the command line options from the admin database
// and return the results as a map.
func (m *Tester) getCommandLineOptions() (bson.M, error) {
	var result bson.M
	err := m.mongoClient.
		Database("admin").
		RunCommand(m.ctx, bson.D{primitive.E{Key: "getCmdLineOpts", Value: 1}}).
		Decode(&result)

	return result, err
}

// bsonToMap will convert a bson map to a regular map recursively.
// objx does not work when the nested objects are bson.M.
func bsonToMap(m bson.M) map[string]interface{} {
	out := make(map[string]interface{})
	for key, value := range m {
		if subMap, ok := value.(bson.M); ok {
			out[key] = bsonToMap(subMap)
		} else {
			out[key] = value
		}
	}
	return out
}

// StartBackgroundConnectivityTest starts periodically checking connectivity to the MongoDB deployment
// with the defined interval. A cancel function is returned, which can be called to stop testing connectivity.
func (m *Tester) StartBackgroundConnectivityTest(t *testing.T, interval time.Duration, opts ...OptionApplier) func() {
	ctx, cancel := context.WithCancel(m.ctx)
	t.Logf("Starting background connectivity test")

	// start a go routine which will periodically check basic MongoDB connectivity
	go func() { //nolint
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				m.ConnectivitySucceeds(opts...)(t)
			}
		}
	}()

	return func() {
		cancel()
		if t != nil {
			t.Log("TestContext cancelled, no longer checking connectivity")
		}
	}
}

// ensureClient establishes a mongo client connection applying any addition
// client options on top of what were provided at construction.
func (t *Tester) ensureClient(ctx context.Context, opts ...*options.ClientOptions) error {
	allOpts := t.clientOpts
	allOpts = append(allOpts, opts...)
	mongoClient, err := mongo.Connect(ctx, allOpts...)
	if err != nil {
		return err
	}
	t.mongoClient = mongoClient
	return nil
}

// PrometheusEndpointIsReachable returns a testing function that will check for
// the Prometheus endpoint to be rechable. It can be configued to use HTTPS if
// `useTls` is set to `true`.
func (m *Tester) PrometheusEndpointIsReachable(username, password string, useTls bool) func(t *testing.T) {
	scheme := "http"
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	if useTls {
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint
		scheme = "https"
	}
	client := &http.Client{Transport: customTransport}

	return func(t *testing.T) {
		_ = wait.PollUntilContextTimeout(m.ctx, 5*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
			var idx int

			// Verify that the Prometheus port is enabled and responding with 200
			// on every Pod.
			for idx = 0; idx < m.resource.Spec.Members; idx++ {
				url := fmt.Sprintf("%s://%s-%d.%s-svc.%s.svc.cluster.local:9216/metrics", scheme, m.resource.Name, idx, m.resource.Name, m.resource.Namespace)
				req, err := http.NewRequest("GET", url, nil)
				assert.NoError(t, err)
				req.SetBasicAuth(username, password)

				response, err := client.Do(req)
				assert.NoError(t, err)
				assert.Equal(t, response.StatusCode, 200)
			}

			return true, nil
		})
	}
}

// clientOptionAdder is the standard implementation that simply adds a
// new options.ClientOption to the mongo client
type clientOptionAdder struct {
	option *options.ClientOptions
}

func (c clientOptionAdder) ApplyOption(opts ...*options.ClientOptions) []*options.ClientOptions {
	return append(opts, c.option)
}

// clientOptionRemover is used if a value from the client array of options should be removed.
// assigning a nil value will not take precedence over an existing value, so we need a mechanism
// to remove elements that are present

// e.g. to disable TLS, you need to remove the options.ClientOption that has a non-nil tls config
// it is not enough to add a tls config that has a nil value.
type clientOptionRemover struct {
	// removalPredicate is a function which returns a bool indicating
	// if a given options.ClientOption should be removed.
	removalPredicate func(opt *options.ClientOptions) bool
}

func (c clientOptionRemover) ApplyOption(opts ...*options.ClientOptions) []*options.ClientOptions {
	newOpts := make([]*options.ClientOptions, 0)
	for _, opt := range opts {
		if !c.removalPredicate(opt) {
			newOpts = append(newOpts, opt)
		}
	}
	return newOpts
}

// WithScram provides a configuration option that will configure the MongoDB resource
// with the given username and password
func WithScram(username, password string) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			Auth: &options.Credential{
				AuthMechanism: "SCRAM-SHA-256",
				AuthSource:    "admin",
				Username:      username,
				Password:      password,
			},
		},
	}
}

func WithScramWithAuth(username, password string, authenticationMechanism string) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			Auth: &options.Credential{
				AuthMechanism: authenticationMechanism,
				AuthSource:    "admin",
				Username:      username,
				Password:      password,
			},
		},
	}
}

func WithX509() OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			Auth: &options.Credential{
				AuthMechanism: "MONGODB-X509",
			},
		},
	}
}

// WithHosts configures the hosts of the deployment
func WithHosts(hosts []string) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			Hosts: hosts,
		},
	}
}

// WithTls configures the client to use tls
func WithTls(ctx context.Context, mdb mdbv1.MongoDBCommunity) OptionApplier {
	tlsConfig, err := getClientTLSConfig(ctx, mdb)
	if err != nil {
		panic(fmt.Errorf("could not retrieve TLS config: %s", err))
	}

	return withTls(tlsConfig)
}

func withTls(tls *tls.Config) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			TLSConfig: tls,
		},
	}
}

// WithoutTls will remove the tls configuration
func WithoutTls() OptionApplier {
	return clientOptionRemover{
		removalPredicate: func(opt *options.ClientOptions) bool {
			return opt.TLSConfig != nil
		},
	}
}

// WithURI will add URI connection string
func WithURI(uri string) OptionApplier {
	opt := &options.ClientOptions{}
	opt.ApplyURI(uri)
	return clientOptionAdder{option: opt}
}

// WithReplicaSet will explicitly add a replicaset name
func WithReplicaSet(rsname string) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			ReplicaSet: &rsname,
		},
	}
}

// getClientTLSConfig reads in the tls fixtures
func getClientTLSConfig(ctx context.Context, mdb mdbv1.MongoDBCommunity) (*tls.Config, error) {
	caSecret := corev1.Secret{}
	caSecretName := types.NamespacedName{Name: mdb.Spec.Security.TLS.CaCertificateSecret.Name, Namespace: mdb.Namespace}
	if err := e2eutil.TestClient.Get(ctx, caSecretName, &caSecret); err != nil {
		return nil, err
	}
	caPEM := caSecret.Data["ca.crt"]
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)
	return &tls.Config{ //nolint
		RootCAs: caPool,
	}, nil

}

// GetAgentCert reads the agent key certificate
func GetAgentCert(ctx context.Context, mdb mdbv1.MongoDBCommunity) (*x509.Certificate, error) {
	certSecret := corev1.Secret{}
	certSecretName := mdb.AgentCertificateSecretNamespacedName()
	if err := e2eutil.TestClient.Get(ctx, certSecretName, &certSecret); err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certSecret.Data["tls.crt"])
	if block == nil {
		return nil, fmt.Errorf("error decoding client cert key")
	}
	return x509.ParseCertificate(block.Bytes)
}

// GetClientCert reads the client key certificate
func GetClientCert(ctx context.Context, mdb mdbv1.MongoDBCommunity) (*x509.Certificate, error) {
	certSecret := corev1.Secret{}
	certSecretName := types.NamespacedName{Name: mdb.Spec.Security.TLS.CertificateKeySecret.Name, Namespace: mdb.Namespace}
	if err := e2eutil.TestClient.Get(ctx, certSecretName, &certSecret); err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certSecret.Data["tls.crt"])
	if block == nil {
		return nil, fmt.Errorf("error decoding client cert key")
	}
	return x509.ParseCertificate(block.Bytes)
}

func GetUserCert(ctx context.Context, mdb mdbv1.MongoDBCommunity, userCertSecret string) (string, error) {
	certSecret := corev1.Secret{}
	certSecretName := types.NamespacedName{Name: userCertSecret, Namespace: mdb.Namespace}
	if err := e2eutil.TestClient.Get(ctx, certSecretName, &certSecret); err != nil {
		return "", err
	}
	crt, _ := pem.Decode(certSecret.Data["tls.crt"])
	if crt == nil {
		return "", fmt.Errorf("error decoding client cert key")
	}
	key, _ := pem.Decode(certSecret.Data["tls.key"])
	if key == nil {
		return "", fmt.Errorf("error decoding client cert key")
	}
	return string(crt.Bytes) + string(key.Bytes), nil
}

// defaults returns the default connectivity options
// that our used in our tests.
// TODO: allow these to be configurable
func defaults() connectivityOpts {
	return connectivityOpts{
		IntervalTime:   1 * time.Second,
		TimeoutTime:    30 * time.Second,
		ContextTimeout: 10 * time.Minute,
		Database:       "testing",
		Collection:     "numbers",
	}
}

type connectivityOpts struct {
	Retries        int
	IntervalTime   time.Duration
	TimeoutTime    time.Duration
	ContextTimeout time.Duration
	Database       string
	Collection     string
}
