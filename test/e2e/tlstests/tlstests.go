package tlstests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnableTLS will upgrade an existing TLS cluster to use TLS.
func EnableTLS(mdb *v1.MongoDB, optional bool) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *v1.MongoDB) {
			db.Spec.Security.TLS = e2eutil.NewTestTLSConfig(optional)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// ConnectivityWithTLS returns a test function which performs
// a basic MongoDB connectivity test over TLS
func ConnectivityWithTLS(mdb *v1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		tlsConfig, err := getClientTLSConfig()
		if err != nil {
			t.Fatal(err)
			return
		}

		if err := mongodbtests.Connect(mdb, options.Client().SetTLSConfig(tlsConfig)); err != nil {
			t.Fatal(fmt.Sprintf("Error connecting to MongoDB deployment over TLS: %+v", err))
		}
	}
}

// ConnectivityWithoutTLSShouldFail will send a single non-TLS query
// and expect it to fail.
func ConnectivityWithoutTLSShouldFail(mdb *v1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		err := connectWithoutTLS(mdb)
		assert.Error(t, err, "expected connectivity test to fail without TLS")
	}
}

// connectWithoutTLS will initialize a single MongoDB client and
// send a single request. This function is used to ensure non-TLS
// requests fail.
func connectWithoutTLS(mdb *v1.MongoDB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mdb.MongoURI()))
	if err != nil {
		return err
	}

	collection := mongoClient.Database("testing").Collection("numbers")
	_, err = collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
	return err
}

// IsReachableDuring periodically tests connectivity to the provided MongoDB resource
// during execution of the provided functions. This function can be used to ensure
// The MongoDB is up throughout the test.
func IsReachableOverTLSDuring(mdb *v1.MongoDB, interval time.Duration, testFunc func()) func(*testing.T) {
	return mongodbtests.IsReachableDuringWithConnection(mdb, interval, testFunc, func() error {
		tlsConfig, err := getClientTLSConfig()
		if err != nil {
			return err
		}

		return mongodbtests.Connect(mdb, options.Client().SetTLSConfig(tlsConfig))
	})
}

// WaitForTLSMode will poll the admin database and wait for the TLS mode to reach a certain value.
func WaitForTLSMode(mdb *v1.MongoDB, expectedValue string) func(*testing.T) {
	return func(t *testing.T) {
		err := wait.Poll(time.Second*10, time.Minute*10, func() (done bool, err error) {
			// Once we upgrade the tests to 4.2 we will have to change this to "tlsMode".
			// We will also have to change the values we check for.
			value, err := getAdminSetting(mdb.MongoURI(), "sslMode")
			if err != nil {
				return false, err
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
func getAdminSetting(uri, key string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tlsConfig, err := getClientTLSConfig()
	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, options.Client().SetTLSConfig(tlsConfig).ApplyURI(uri))
	if err != nil {
		return "", err
	}

	var result bson.D
	client.
		Database("admin").
		RunCommand(ctx, bson.D{{"getParameter", 1}, {key, 1}}).
		Decode(&result)

	value := result.Map()[key]
	return value, nil
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
