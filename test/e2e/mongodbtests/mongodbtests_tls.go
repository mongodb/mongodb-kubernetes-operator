package mongodbtests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnableTLS will upgrade an existing TLS cluster to use TLS.
func EnableTLS(mdb *mdbv1.MongoDB, optional bool) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDB) {
			db.Spec.Security.TLS = e2eutil.NewTestTLSConfig(optional)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// BasicConnectivityWithTLS returns a test function which performs
// a basic MongoDB connectivity test over TLS
func BasicConnectivityWithTLS(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		opts := options.Client().SetTLSConfig(getClientTLSConfig())
		if err := connect(mdb, opts); err != nil {
			t.Fatal(fmt.Sprintf("Error connecting to MongoDB deployment over TLS: %+v", err))
		}
	}
}

// EnsureTLSIsRequired will send a single non-TLS query
// and expect it to fail.
func EnsureTLSIsRequired(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		err := connectWithoutTLS(mdb)
		assert.Error(t, err, "expected connectivity test to fail without TLS")
	}
}

// connectWithoutTLS will initialize a single MongoDB client and
// send a single request. This function is used to ensure non-TLS
// requests fail.
func connectWithoutTLS(mdb *mdbv1.MongoDB) error {
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
func IsReachableOverTLSDuring(mdb *mdbv1.MongoDB, interval time.Duration, testFunc func()) func(*testing.T) {
	return isReachableDuring(mdb, interval, testFunc, func() error {
		opts := options.Client().SetTLSConfig(getClientTLSConfig())
		return connect(mdb, opts)
	})
}

// WaitForTLSMode will poll the admin database and wait for the TLS mode to reach a certain value.
func WaitForTLSMode(mdb *mdbv1.MongoDB, expectedValue string) func(*testing.T) {
	return func(t *testing.T) {
		err := wait.Poll(time.Second*10, time.Minute*10, func() (done bool, err error) {
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
func getAdminSetting(url, key string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Client().
		SetTLSConfig(getClientTLSConfig()).
		ApplyURI(url)

	client, err := mongo.Connect(ctx, opts)
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

func getClientTLSConfig() *tls.Config {
	// Read the CA certificate from test data
	caPool := x509.NewCertPool()
	caPEM, _ := ioutil.ReadFile("testdata/tls/ca.crt")
	caPool.AppendCertsFromPEM(caPEM)

	return &tls.Config{
		RootCAs: caPool,
	}
}
