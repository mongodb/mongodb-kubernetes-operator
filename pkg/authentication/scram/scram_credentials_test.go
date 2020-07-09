package scram

import (
	"crypto/sha1"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDeterministicSalt(t *testing.T) {
	t.Run("same name and sha 256 hashing algorithm should result in same salt", func(t *testing.T) {
		assert.Equal(t, getDeterministicSalt("my-mdb", sha256.New), getDeterministicSalt("my-mdb", sha256.New))
	})

	t.Run("same name and sha 1 hashing algorithm should result in same salt", func(t *testing.T) {
		assert.Equal(t, getDeterministicSalt("my-mdb", sha1.New), getDeterministicSalt("my-mdb", sha1.New))
	})

	t.Run("different resource name results in different salt", func(t *testing.T) {
		assert.NotEqual(t, getDeterministicSalt("my-different-mdb", sha256.New), getDeterministicSalt("my-mdb", sha256.New))
	})

	t.Run("using different algorithm results in different salt", func(t *testing.T) {
		assert.NotEqual(t, getDeterministicSalt("my-mdb", sha256.New), getDeterministicSalt("my-mdb", sha1.New))
	})
}
