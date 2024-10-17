package mongotester

import (
	"crypto/tls"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestTlsRemoval_RemovesCorrectConfig(t *testing.T) {
	// configure TLS and hosts
	opts := withTls(&tls.Config{ //nolint
		ServerName: "some-name",
	}).ApplyOption()

	opts = WithHosts([]string{"host1", "host2", "host3"}).ApplyOption(opts...)

	removalOpt := WithoutTls()

	// remove the tls value
	opts = removalOpt.ApplyOption(opts...)

	var errs []error
	raw := options.ClientOptions{}
	for _, opt := range opts {
		for _, fn := range opt.List() {
			err := fn(&raw)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	assert.Len(t, errs, 0)
	assert.Len(t, opts, 3, "tls removal should remove an element")
	assert.NotNil(t, raw.Hosts, "tls removal should not effect other configs")
	assert.Len(t, raw.Hosts, 3, "original configs should not be changed")
	assert.True(t, reflect.DeepEqual(raw.Hosts, []string{"host1", "host2", "host3"}))
}

func TestWithScram_AddsScramOption(t *testing.T) {
	opts := WithScram("username", "password").ApplyOption()

	raw := options.ClientOptions{}
	var errs []error
	for _, opt := range opts {
		for _, fn := range opt.List() {
			err := fn(&raw)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	assert.Len(t, errs, 0)
	assert.Len(t, opts, 1)
	assert.NotNil(t, opts)
	assert.Equal(t, raw.Auth.AuthMechanism, "SCRAM-SHA-256")
	assert.Equal(t, raw.Auth.Username, "username")
	assert.Equal(t, raw.Auth.Password, "password")
	assert.Equal(t, raw.Auth.AuthSource, "admin")
}
