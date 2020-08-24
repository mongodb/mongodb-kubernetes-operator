package tlstests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/util/wait"

	f "github.com/operator-framework/operator-sdk/pkg/test"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
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
func ConnectivityWithTLS(mdb *v1.MongoDB, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		tlsConfig, err := getClientTLSConfig()
		if err != nil {
			t.Fatal(err)
			return
		}

		if err := mongodbtests.Connect(mdb, options.Client().SetTLSConfig(tlsConfig).SetAuth(options.Credential{
			AuthMechanism: "SCRAM-SHA-256",
			Username:      username,
			Password:      password,
		})); err != nil {
			t.Fatal(fmt.Sprintf("Error connecting to MongoDB deployment over TLS: %+v", err))
		}
	}
}

// ConnectivityWithoutTLSShouldFail will send a single non-TLS query
// and expect it to fail.
func ConnectivityWithoutTLSShouldFail(mdb *v1.MongoDB, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		err := connectWithoutTLS(mdb, username, password)
		assert.Error(t, err, "expected connectivity test to fail without TLS")
	}
}

// connectWithoutTLS will initialize a single MongoDB client and
// send a single request. This function is used to ensure non-TLS
// requests fail.
func connectWithoutTLS(mdb *v1.MongoDB, username, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mdb.SCRAMMongoURI(username, password)))
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
func IsReachableOverTLSDuring(mdb *v1.MongoDB, interval time.Duration, username, password string, testFunc func()) func(*testing.T) {
	return mongodbtests.IsReachableDuringWithConnection(mdb, interval, testFunc, func() error {
		tlsConfig, err := getClientTLSConfig()
		if err != nil {
			return err
		}

		return mongodbtests.Connect(mdb, options.Client().
			SetTLSConfig(tlsConfig).
			SetAuth(options.Credential{
				AuthMechanism: "SCRAM-SHA-256",
				Username:      username,
				Password:      password,
			}))
	})
}

// WaitForTLSMode will poll the admin database and wait for the TLS mode to reach a certain value.
func WaitForTLSMode(mdb *v1.MongoDB, expectedValue, username, password string) func(*testing.T) {
	return func(t *testing.T) {
		err := wait.Poll(time.Second*10, time.Minute*10, func() (done bool, err error) {
			// Once we upgrade the tests to 4.2 we will have to change this to "tlsMode".
			// We will also have to change the values we check for.
			value, err := getAdminSetting(mdb.SCRAMMongoURI(username, password), "sslMode")
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

func RotateCertificate(mdb *v1.MongoDB) func(*testing.T) {
	return func(t *testing.T) {
		// Load new certificate and key
		cert, err := ioutil.ReadFile("testdata/tls/server_rotated.crt")
		assert.NoError(t, err)
		key, err := ioutil.ReadFile("testdata/tls/server_rotated.key")
		assert.NoError(t, err)

		certKeySecret := secret.Builder().
			SetName(mdb.Spec.Security.TLS.CertificateKeySecret.Name).
			SetNamespace(mdb.Namespace).
			SetField("tls.crt", string(cert)).
			SetField("tls.key", string(key)).
			Build()

		err = f.Global.Client.Update(context.TODO(), &certKeySecret)
		assert.NoError(t, err)
	}
}

func WaitForRotatedCertificate(mdb *v1.MongoDB) func(*testing.T) {
	return func(t *testing.T) {
		// The rotated certificate has serial number 2
		expectedSerial := big.NewInt(2)

		tlsConfig, err := getClientTLSConfig()
		assert.NoError(t, err)

		// Reject all server certificates that don't have the expected serial number
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert := verifiedChains[0][0]
			if expectedSerial.Cmp(cert.SerialNumber) != 0 {
				return fmt.Errorf("expected certificate serial number %s, got %s", expectedSerial, cert.SerialNumber)
			}

			return nil
		}

		opts := options.Client().SetTLSConfig(tlsConfig).ApplyURI(mdb.MongoURI())
		mongoClient, err := mongo.Connect(context.TODO(), opts)
		assert.NoError(t, err)

		// Ping the cluster until it succeeds. The ping will only suceed with the right certificate.
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			if err := mongoClient.Ping(context.TODO(), nil); err != nil {
				return false, nil
			}
			return true, nil
		})
		assert.NoError(t, err)
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
