package scramcredentials

import (
	"crypto/sha1" //nolint
	"crypto/sha256"
	"hash"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScramSha1SecretsMatch(t *testing.T) {
	assertSecretsMatch(t, sha1.New, "caeec61ba3b15b15b188d29e876514e8", 10, "S3cuk2Rnu/MlbewzxrmmVA==", "sYBa3XlSPKNrgjzhOuEuRlJY4dQ=", "zuAxRSQb3gZkbaB1IGlusK4jy1M=")
	assertSecretsMatch(t, sha1.New, "4d9625b297999b3ca786d4a9622d04f1", 10, "kW9KbCQiCOll5Ljd44cjkQ==", "VJ8fFVHkPltibvT//mG/OWw44Hc=", "ceDRsgj9HezpZ4/vkZX8GZNNN50=")
	assertSecretsMatch(t, sha1.New, "fd0a78e418dcef39f8c768222810b894", 10, "hhX6xsoID6FeWjXncuNgAg==", "TxgaZJ4cIn+S9EfTcc9IOEG7RGc=", "d6/qjwBs0qkPKfUAjSh5eemsySE=")
}
func TestScramSha256SecretsMatch(t *testing.T) {
	assertSecretsMatch(t, sha256.New, "Gy4ZNMr-SYEsEpAEZv", 15000, "ajdf1E1QTsNAQdBEodB4vzQOFuvcw9K6PmouVg==", "/pBk9XBwSm9UyeQmyJ3LfogfHu9Z/XTjGmRhQDHx/4I=", "Avm8mjtMyg659LAyeD4VmuzQb5lxL5iy3dCuzfscfMc=")
	assertSecretsMatch(t, sha256.New, "Y9SPYSJYUJB_", 15000, "Oplsu3uju+lYyX4apKb0K6xfHpmFtH99Oyk4Ow==", "oTJhml8KKZUSt9k4tg+tS6D/ygR+a2Xfo8JKjTpQoAI=", "SUfA2+SKL35u665WY5NnJJmA9L5dHu/TnWXX/0nm42Y=")
	assertSecretsMatch(t, sha256.New, "157VDZr0h-Pz-wj72", 15000, "P/4xs3anygxu3/l2p35CSBe4Z47IV/FtE/e44A==", "jOb27nFF72SQoY7WUqKXOTR4e8jETXxMS67SONrcbjA=", "3FnslkgUweautAfPRCOEjhS+YbUYUNmdDQUGxB+oaFE=")
	assertSecretsMatch(t, sha256.New, "P8z1sDfELCePTNbVqX", 15000, "RPNhenwTHlqW5OE597XpuwvPLaiecPpYFa58Pg==", "sJ8UhQRszLNo15cOe62+HLjt2NxmSkJGjdJpclTIMBs=", "CSg02ODAvh9+swUHoimXcDsT9lLp/A5IhQXavXl7+qA=")
}

func assertSecretsMatch(t *testing.T, hash func() hash.Hash, passwordHash string, iterationCount int, salt, storedKey, serverKey string) {
	computedStoredKey, computedServerKey, err := generateB64EncodedSecrets(hash, passwordHash, salt, iterationCount)
	assert.NoError(t, err)
	assert.Equal(t, computedStoredKey, storedKey)
	assert.Equal(t, computedServerKey, serverKey)
}
