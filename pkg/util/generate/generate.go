package generate

import (
	"crypto/rand"
	"crypto/sha1" // nolint
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"unicode"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
)

// final key must be between 6 and at most 1024 characters
func KeyFileContents() (string, error) {
	return generateRandomString(500)
}

// RandomValidDNS1123Label generates a random fixed-length string with characters in a certain range.
func RandomValidDNS1123Label(n int) (string, error) {
	str, err := RandomFixedLengthStringOfSize(n)
	if err != nil {
		return "", err
	}

	runes := []rune(str)

	// Make sure that any letters are lowercase and that if any non-alphanumeric characters appear they are set to '0'.
	for i, r := range runes {
		if unicode.IsLetter(r) {
			runes[i] = unicode.ToLower(r)
		} else if !unicode.IsNumber(r) {
			runes[i] = rune('0')
		}
	}

	return string(runes), nil
}

func RandomFixedLengthStringOfSize(n int) (string, error) {
	b, err := generateRandomBytes(n)
	return base64.URLEncoding.EncodeToString(b)[:n], err
}

// Salts generates 2 different salts. The first is for the sha1 algorithm
// the second is for sha256
func Salts() ([]byte, []byte, error) {
	sha1Salt, err := salt(sha1.New)
	if err != nil {
		return nil, nil, err
	}

	sha256Salt, err := salt(sha256.New)
	if err != nil {
		return nil, nil, err
	}
	return sha1Salt, sha256Salt, nil
}

// salt will create a salt which can be used to compute Scram Sha credentials based on the given hashConstructor.
// sha1.New should be used for MONGODB-CR/SCRAM-SHA-1 and sha256.New should be used for SCRAM-SHA-256
func salt(hashConstructor func() hash.Hash) ([]byte, error) {
	saltSize := hashConstructor().Size() - scramcredentials.RFC5802MandatedSaltSize
	salt, err := RandomFixedLengthStringOfSize(20)

	if err != nil {
		return nil, err
	}
	shaBytes32 := sha256.Sum256([]byte(salt))

	// the algorithms expect a salt of a specific size.
	return shaBytes32[:saltSize], nil
}

func generateRandomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func generateRandomString(numBytes int) (string, error) {
	b, err := generateRandomBytes(numBytes)
	return base64.StdEncoding.EncodeToString(b), err
}
