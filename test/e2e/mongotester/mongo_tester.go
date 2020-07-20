package mongotester

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var tlsConfig *tls.Config = nil

func initTls() {
	var err error
	tlsConfig, err = getClientTLSConfig()
	if err != nil {
		os.Exit(1)
	}
}

type Tester struct {
	mongoClient *mongo.Client
	clientOpts  []*options.ClientOptions
	mdb         *mdbv1.MongoDB
	tls         tls.Config
}

func NewTester(opts ...*options.ClientOptions) *Tester {
	initTls()
	t := &Tester{}
	for _, opt := range opts {
		t.clientOpts = append(t.clientOpts, opt)
	}
	return t
}

func FromResource(t *testing.T, mdb mdbv1.MongoDB, opts ...*options.ClientOptions) (*Tester, error) {
	var clientOpts []*options.ClientOptions
	clientOpts = append(clientOpts, WithHosts(mdb.Hosts()))
	t.Logf("Configuring hosts: %s for MongoDB: %s", mdb.Hosts(), mdb.NamespacedName())

	//if mdb.Spec.Security.TLS.Enabled {
	//	tlsConfig, err := getClientTLSConfig()
	//	if err != nil {
	//		return nil, err
	//	}
	//	clientOpts = append(clientOpts, &options.ClientOptions{
	//		TLSConfig: tlsConfig,
	//	})
	//}
	users := mdb.Spec.Users
	if len(users) == 1 {
		user := users[0]
		passwordSecret := corev1.Secret{}
		err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: f.Global.OperatorNamespace}, &passwordSecret)
		if err != nil {
			return nil, err
		}
		t.Logf("Configuring SCRAM username: %s and password from secret %s for MongoDB: %s", user.Name, user.PasswordSecretRef.Name, mdb.NamespacedName())
		clientOpts = append(clientOpts, WithScram(user.Name, string(passwordSecret.Data[user.PasswordSecretRef.Key])))
	}

	// add any additional options
	clientOpts = append(clientOpts, opts...)

	return NewTester(clientOpts...), nil
}

func (m *Tester) ConnectivitySucceeds(opts ...*options.ClientOptions) func(t *testing.T) {
	return m.connectivityCheck(true, opts...)
}

func (m *Tester) ConnectivityFails(opts ...*options.ClientOptions) func(t *testing.T) {
	return m.connectivityCheck(false, opts...)
}

func (m *Tester) connectivityCheck(shouldSucceed bool, opts ...*options.ClientOptions) func(t *testing.T) {
	connectivityOpts := defaults()
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), connectivityOpts.ContextTimeout)
		defer cancel()

		if err := m.ensureClient(opts...); err != nil {
			t.Fatal(err)
		}

		collection := m.mongoClient.Database(connectivityOpts.Database).Collection(connectivityOpts.Collection)
		_, err := collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
		if err != nil && shouldSucceed {
			t.Fatal(err)
			return
		}
		if err == nil && !shouldSucceed {
			t.Fatal(fmt.Sprintf("Was successfully able to connect, when we should not have been able to!"))
		}
	}
}

// HasFeatureCompatibilityVersion verifies that the FeatureCompatibilityVersion is
// set to `version`. The FCV parameter is not signaled as a non Running state, for
// this reason, this function checks the value of the parameter many times, based
// on the value of `tries`.
func (m *Tester) HasFeatureCompatibilityVersion(fcv string, tries int) func(t *testing.T) {
	//connectivityOpts := connectivity.Defaults()
	connectivityOpts := defaults()
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), connectivityOpts.ContextTimeout)
		defer cancel()

		if err := m.ensureClient(); err != nil {
			t.Fatal(err)
		}

		database := m.mongoClient.Database("admin")
		assert.NotNil(t, database)
		runCommand := bson.D{
			primitive.E{Key: "getParameter", Value: 1},
			primitive.E{Key: "featureCompatibilityVersion", Value: 1},
		}
		found := false
		for !found && tries > 0 {
			<-time.After(10 * time.Second)
			var result bson.M
			if err := database.RunCommand(ctx, runCommand).Decode(&result); err != nil {
				continue
			}
			expected := primitive.M{"version": fcv}
			if reflect.DeepEqual(expected, result["featureCompatibilityVersion"]) {
				found = true
			}

			tries--
		}
		assert.True(t, found)
	}
}

// WaitForTLSMode will poll the admin database and wait for the TLS mode to reach a certain value.
func (m *Tester) WaitForTLSMode(expectedValue string, opts ...*options.ClientOptions) func(*testing.T) {
	return func(t *testing.T) {
		if err := m.ensureClient(opts...); err != nil {
			t.Fatal(err)
		}
		err := wait.Poll(time.Second*10, time.Minute*10, func() (done bool, err error) {
			// Once we upgrade the tests to 4.2 we will have to change this to "tlsMode".
			// We will also have to change the values we check for.
			value, err := m.getAdminSetting("sslMode")
			if err != nil {
				t.Logf("error getting admin setting: %s", err)
				return false, nil
			}

			if value != expectedValue {
				return false, nil
			}

			return true, nil
		})

		if err != nil {
			t.Fatal(fmt.Sprintf(`Error waiting for TLS mode to reach "%s": %+v`, expectedValue, err))
		}
	}
}

// getAdminSetting will get a setting from the admin database.
func (m *Tester) getAdminSetting(key string) (interface{}, error) {
	var result bson.D
	err := m.mongoClient.
		Database("admin").
		RunCommand(context.TODO(), bson.D{{"getParameter", 1}, {key, 1}}).
		Decode(&result)
	if err != nil {
		return nil, err
	}
	value := result.Map()[key]
	return value, nil
}

// StartBackgroundConnectivityTest starts periodically checking connectivity to the MongoDB deployment
// with the defined interval. A cancel function is returned, which can be called to stop testing connectivity.
func (m *Tester) StartBackgroundConnectivityTest(t *testing.T, interval time.Duration) func() {
	ctx, cancel := context.WithCancel(context.Background()) // start a go routine which will periodically check basic MongoDB connectivity
	// once all the test functions have been executed, the go routine will be cancelled
	t.Logf("Starting background connectivity test")

	go func() { //nolint
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				m.ConnectivitySucceeds()(t)
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

func WithScram(username, password string) *options.ClientOptions {
	return &options.ClientOptions{
		Auth: &options.Credential{
			AuthMechanism: "SCRAM-SHA-256", // TODO: handle SCRAM-SHA-1
			AuthSource:    "admin",
			Username:      username,
			Password:      password,
		},
	}
}

func WithHosts(hosts []string) *options.ClientOptions {
	return &options.ClientOptions{
		Hosts: hosts,
	}
}

func WithTls() *options.ClientOptions {
	return &options.ClientOptions{
		TLSConfig: tlsConfig,
	}
}

func WithoutTls() *options.ClientOptions {
	return &options.ClientOptions{
		TLSConfig: nil,
	}
}

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
