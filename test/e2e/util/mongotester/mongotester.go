package mongotester

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/big"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/objx"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/pkg/errors"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// tlsConfig is the test tls fixture that we use for testing
// tls.
var tlsConfig *tls.Config = nil

// initTls loads in the tls configuration fixture
func initTls() error {
	if tlsConfig != nil {
		return nil
	}
	var err error
	tlsConfig, err = getClientTLSConfig()
	if err != nil {
		return err
	}
	return nil
}

type Tester struct {
	mongoClient *mongo.Client
	clientOpts  []*options.ClientOptions
}

func newTester(opts ...*options.ClientOptions) (*Tester, error) {
	if err := initTls(); err != nil {
		return nil, err
	}

	t := &Tester{}
	t.clientOpts = append(t.clientOpts, opts...)
	return t, nil
}

// OptionApplier is an interface which is able to accept a list
// of options.ClientOptions, and return the final desired list
// making any modifications required
type OptionApplier interface {
	ApplyOption(opts ...*options.ClientOptions) []*options.ClientOptions
}

// FromResource returns a Tester instance from a MongoDB resource. It infers SCRAM username/password
// and the hosts from the resource.
func FromResource(t *testing.T, mdb mdbv1.MongoDBCommunity, opts ...OptionApplier) (*Tester, error) {
	var clientOpts []*options.ClientOptions

	clientOpts = WithHosts(mdb.Hosts()).ApplyOption(clientOpts...)

	t.Logf("Configuring hosts: %s for MongoDB: %s", mdb.Hosts(), mdb.NamespacedName())

	users := mdb.Spec.Users
	if len(users) == 1 {
		user := users[0]
		passwordSecret := corev1.Secret{}
		err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: mdb.Namespace}, &passwordSecret)
		if err != nil {
			return nil, err
		}
		t.Logf("Configuring SCRAM username: %s and password from secret %s for MongoDB: %s", user.Name, user.PasswordSecretRef.Name, mdb.NamespacedName())
		clientOpts = WithScram(user.Name, string(passwordSecret.Data[user.PasswordSecretRef.Key])).ApplyOption(clientOpts...)
	}

	if mdb.Spec.Security.TLS.Enabled {
		clientOpts = WithTls().ApplyOption(clientOpts...)
	}

	// add any additional options
	for _, opt := range opts {
		clientOpts = opt.ApplyOption(clientOpts...)
	}

	return newTester(clientOpts...)
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

func (m *Tester) HasKeyfileAuth(tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("clusterAuthMode", "keyFile", tries, opts...)
}

func (m *Tester) HasFCV(fcv string, tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("featureCompatibilityVersion", map[string]interface{}{"version": fcv}, tries, opts...)
}

func (m *Tester) ScramIsConfigured(tries int, opts ...OptionApplier) func(t *testing.T) {
	return m.hasAdminParameter("authenticationMechanisms", primitive.A{"SCRAM-SHA-256"}, tries, opts...)
}

func (m *Tester) EnsureAuthenticationIsConfigured(tries int, opts ...OptionApplier) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("Ensure keyFile authentication is configured", m.HasKeyfileAuth(tries, opts...))
		t.Run("SCRAM-SHA-256 is configured", m.ScramIsConfigured(tries, opts...))
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
			RunCommand(context.TODO(),
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
		if err := m.ensureClient(clientOpts...); err != nil {
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
			RunCommand(context.TODO(), bson.D{{Key: "getParameter", Value: 1}, {Key: key, Value: 1}}).
			Decode(&result)
		actualValue := result[key]
		t.Logf("Actual Value: %+v, type: %s", actualValue, reflect.TypeOf(actualValue))
		if err != nil {
			t.Logf("Unable to get admin setting %s with error : %s", key, err)
			return false
		}
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
		ctx, cancel := context.WithTimeout(context.Background(), connectivityOpts.ContextTimeout)
		defer cancel()

		if err := m.ensureClient(clientOpts...); err != nil {
			t.Fatal(err)
		}

		attempts := 0
		// There can be a short time before the user can auth as the user
		err := wait.Poll(connectivityOpts.IntervalTime, connectivityOpts.TimeoutTime, func() (done bool, err error) {
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
			t.Logf("Connectivity check was successful after %d attempt(s)", attempts)
			return true, nil
		})

		if err != nil {
			t.Fatal(fmt.Errorf("error during connectivity check: %s", err))
		}
	}
}

func (m *Tester) WaitForRotatedCertificate() func(*testing.T) {
	return func(t *testing.T) {
		// The rotated certificate has serial number 2
		expectedSerial := big.NewInt(2)

		tls, err := getClientTLSConfig()
		assert.NoError(t, err)

		// Reject all server certificates that don't have the expected serial number
		tls.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert := verifiedChains[0][0]
			if expectedSerial.Cmp(cert.SerialNumber) != 0 {
				return errors.Errorf("expected certificate serial number %s, got %s", expectedSerial, cert.SerialNumber)
			}
			return nil
		}

		if err := m.ensureClient(&options.ClientOptions{TLSConfig: tls}); err != nil {
			t.Fatal(err)
		}

		// Ping the cluster until it succeeds. The ping will only succeed with the right certificate.
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			if err := m.mongoClient.Ping(context.TODO(), nil); err != nil {
				return false, nil
			}
			return true, nil
		})
		assert.NoError(t, err)
	}
}

func (m *Tester) EnsureMongodConfig(selector string, expected interface{}) func(*testing.T) {
	return func(t *testing.T) {
		opts, err := m.getCommandLineOptions()
		assert.NoError(t, err)

		// The options are stored under the key "parsed"
		parsed := objx.New(bsonToMap(opts)).Get("parsed").ObjxMap()
		assert.Equal(t, expected, parsed.Get(selector).Data())
	}
}

// getCommandLineOptions will get the command line options from the admin database
// and return the results as a map.
func (m *Tester) getCommandLineOptions() (bson.M, error) {
	var result bson.M
	err := m.mongoClient.
		Database("admin").
		RunCommand(context.TODO(), bson.D{primitive.E{Key: "getCmdLineOpts", Value: 1}}).
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
	ctx, cancel := context.WithCancel(context.Background())
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
			t.Log("Context cancelled, no longer checking connectivity")
		}
	}
}

// ensureClient establishes a mongo client connection applying any addition
// client options on top of what were provided at construction.
func (t *Tester) ensureClient(opts ...*options.ClientOptions) error {
	allOpts := t.clientOpts
	allOpts = append(allOpts, opts...)
	mongoClient, err := mongo.Connect(context.TODO(), allOpts...)
	if err != nil {
		return err
	}
	t.mongoClient = mongoClient
	return nil
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

// WithHosts configures the hosts of the deployment
func WithHosts(hosts []string) OptionApplier {
	return clientOptionAdder{
		option: &options.ClientOptions{
			Hosts: hosts,
		},
	}
}

// WithTls configures the client to use tls
func WithTls() OptionApplier {
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

// getClientTLSConfig reads in the tls fixtures
func getClientTLSConfig() (*tls.Config, error) {
	// Read the CA certificate from test data
	caPEM, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "ca.crt"))
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	return &tls.Config{ //nolint
		RootCAs: caPool,
	}, nil
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
