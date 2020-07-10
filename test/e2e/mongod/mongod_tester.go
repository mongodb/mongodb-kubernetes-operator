package mongod

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/connectivity"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Tester struct {
	mongoClient *mongo.Client
	clientOpts  []*options.ClientOptions
}

func (t *Tester) ensureClient() error {
	mongoClient, err := mongo.Connect(context.TODO(), t.clientOpts...)
	if err != nil {
		return err
	}
	t.mongoClient = mongoClient
	return nil
}

func (tt Tester) BasicConnectivity(opts ...connectivity.Modification) func(t *testing.T) {
	connectivityOpts := connectivity.New(opts...)
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), connectivityOpts.ContextTimeout)
		defer cancel()

		if err := tt.ensureClient(); err != nil {
			t.Fatal(err)
		}

		err := wait.Poll(connectivityOpts.IntervalTime, connectivityOpts.TimeoutTime, func() (done bool, err error) {
			collection := tt.mongoClient.Database("testing").Collection("numbers")
			_, err = collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
			if err != nil {
				t.Logf("error inserting document: %s", err)
				return false, nil
			}
			return true, nil
		})

		if err != nil {
			t.Fatal(err)
		}
	}
}

func NewTester(opts ...*options.ClientOptions) *Tester {
	t := &Tester{}
	for _, opt := range opts {
		t.clientOpts = append(t.clientOpts, opt)
	}
	return t
}

func FromMongoDBResource(mdb mdbv1.MongoDB, username, password string, opts ...*options.ClientOptions) (*Tester, error) {
	var clientOpts []*options.ClientOptions
	clientOpts = append(clientOpts, WithHosts(mdb.Hosts()))
	clientOpts = append(clientOpts, WithScram(username, password))
	if mdb.Spec.Security.TLS.Enabled {
		certPool, err := getClientTLSConfig()
		if err != nil {
			return nil, err
		}
		clientOpts = append(clientOpts, WithTLS(certPool))
	}

	// add any additional options
	clientOpts = append(clientOpts, opts...)

	return NewTester(clientOpts...), nil
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

func WithTLS(tlsConfig *tls.Config) *options.ClientOptions {
	return &options.ClientOptions{
		TLSConfig: tlsConfig,
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
