package mongotester

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
// NOTE: Tls is not configured as the mechanism that the ClientOptions are merged only merge on non-nil
// values, meaning we need to remove option that configures TLS from the list if we want to not use tls.
// For now we can just explicitly pass WithTls() or WithoutTls() to configure TLS.
func FromResource(t *testing.T, mdb mdbv1.MongoDB, opts ...OptionApplier) (*Tester, error) {
	var clientOpts []*options.ClientOptions

	clientOpts = WithHosts(mdb.Hosts()).ApplyOption(clientOpts...)

	t.Logf("Configuring hosts: %s for MongoDB: %s", mdb.Hosts(), mdb.NamespacedName())

	users := mdb.Spec.Users
	if len(users) == 1 {
		user := users[0]
		passwordSecret := corev1.Secret{}
		err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: f.Global.OperatorNamespace}, &passwordSecret)
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

	return newTester(clientOpts...)
}

// ConnectivitySucceeds performs a basic check that ensures that it is possible
// to connect to the MongoDB resource
func (m *Tester) ConnectivitySucceeds(opts ...OptionApplier) func(t *testing.T) {
	return m.connectivityCheck(true, opts...)
}

// ConnectivitySucceeds performs a basic check that ensures that it is not possible
// to connect to the MongoDB resource
func (m *Tester) ConnectivityFails(opts ...OptionApplier) func(t *testing.T) {
	return m.connectivityCheck(false, opts...)
}

func (m *Tester) HasKeyfileAuth(tries int) func(t *testing.T) {
	return m.hasAdminParameter("clusterAuthMode", "keyFile", tries)
}

func (m *Tester) HasFCV(fcv string, tries int) func(t *testing.T) {
	return m.hasAdminParameter("featureCompatibilityVersion", map[string]interface{}{"version": fcv}, tries)
}

func (m *Tester) HasTlsMode(tlsMode string, tries int) func(t *testing.T) {
	return m.hasAdminParameter("sslMode", tlsMode, tries)
}

func (m *Tester) hasAdminParameter(key string, expectedValue interface{}, tries int) func(t *testing.T) {
	return func(t *testing.T) {
		if err := m.ensureClient(); err != nil {
			t.Fatal(err)
		}

		database := m.mongoClient.Database("admin")
		assert.NotNil(t, database)

		found := false
		for !found && tries > 0 {
			<-time.After(10 * time.Second)
			actualValue, err := m.getAdminSetting(key)
			t.Logf("Actual Value: %+v", actualValue)
			if err != nil {
				t.Logf("Unable to get admin setting %s with error : %s", key, err)
				continue
			}
			found = reflect.DeepEqual(expectedValue, actualValue)
			tries--
		}
		assert.True(t, found)
	}
}

// getAdminSetting will get a setting from the admin database.
func (m *Tester) getAdminSetting(key string) (interface{}, error) {
	var result map[string]interface{}
	err := m.mongoClient.
		Database("admin").
		RunCommand(context.TODO(), bson.D{{Key: "getParameter", Value: 1}, {Key: key, Value: 1}}).
		Decode(&result)
	if err != nil {
		return nil, err
	}
	value := result[key]
	return value, nil
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
// assigning a nil vlaue will not take precedence over an existing value, so we need a mechanism
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
// resulting in using a TLS tls connection
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
	caPEM, err := ioutil.ReadFile("testdata/tls/ca.crt")
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	return &tls.Config{
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
