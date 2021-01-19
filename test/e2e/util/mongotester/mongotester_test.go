package mongotester

import (
	"crypto/tls"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestTlsRemoval_RemovesCorrectConfig(t *testing.T) {
	var opts []*options.ClientOptions

	// configure TLS and hosts
	opts = withTls(&tls.Config{ //nolint
		ServerName: "some-name",
	}).ApplyOption(opts...)
	opts = WithHosts([]string{"host1", "host2", "host3"}).ApplyOption(opts...)

	removalOpt := WithoutTls()

	// remove the tls value
	opts = removalOpt.ApplyOption(opts...)

	assert.Len(t, opts, 1, "tls removal should remove an element")
	assert.NotNil(t, opts[0].Hosts, "tls removal should not effect other configs")
	assert.Len(t, opts[0].Hosts, 3, "original configs should not be changed")
	assert.True(t, reflect.DeepEqual(opts[0].Hosts, []string{"host1", "host2", "host3"}))
}

func TestWithScram_AddsScramOption(t *testing.T) {
	var opts []*options.ClientOptions

	opts = WithScram("username", "password").ApplyOption(opts...)

	assert.Len(t, opts, 1)
	assert.NotNil(t, opts[0])
	assert.Equal(t, opts[0].Auth.AuthMechanism, "SCRAM-SHA-256")
	assert.Equal(t, opts[0].Auth.Username, "username")
	assert.Equal(t, opts[0].Auth.Password, "password")
	assert.Equal(t, opts[0].Auth.AuthSource, "admin")
}
