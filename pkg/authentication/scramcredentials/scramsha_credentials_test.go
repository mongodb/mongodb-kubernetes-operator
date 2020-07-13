package scramcredentials

import (
	"crypto/sha1"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScramSha1SecretsMatch(t *testing.T) {
	// these were taken from MongoDB.  passwordHash is from authSchema
	// 3. iterationCount, salt, storedKey, and serverKey are from
	// authSchema 5 (after upgrading from authSchema 3)
	assertSecretsMatch(t, sha1.New, "caeec61ba3b15b15b188d29e876514e8", 10, "S3cuk2Rnu/MlbewzxrmmVA==", "sYBa3XlSPKNrgjzhOuEuRlJY4dQ=", "zuAxRSQb3gZkbaB1IGlusK4jy1M=")
	assertSecretsMatch(t, sha1.New, "4d9625b297999b3ca786d4a9622d04f1", 10, "kW9KbCQiCOll5Ljd44cjkQ==", "VJ8fFVHkPltibvT//mG/OWw44Hc=", "ceDRsgj9HezpZ4/vkZX8GZNNN50=")
	assertSecretsMatch(t, sha1.New, "fd0a78e418dcef39f8c768222810b894", 10, "hhX6xsoID6FeWjXncuNgAg==", "TxgaZJ4cIn+S9EfTcc9IOEG7RGc=", "d6/qjwBs0qkPKfUAjSh5eemsySE=")
}

func assertSecretsMatch(t *testing.T, hash func() hash.Hash, passwordHash string, iterationCount int, salt, storedKey, serverKey string) {
	computedStoredKey, computedServerKey, err := generateB64EncodedSecrets(hash, passwordHash, salt, iterationCount)
	assert.NoError(t, err)
	assert.Equal(t, computedStoredKey, storedKey)
	assert.Equal(t, computedServerKey, serverKey)
}
