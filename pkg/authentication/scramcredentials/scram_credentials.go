package scramcredentials

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/xdg/stringprep"
)

const (
	RFC5802MandatedSaltSize = 4

	clientKeyInput = "Client Key" // specified in RFC 5802
	serverKeyInput = "Server Key" // specified in RFC 5802

	// using the default MongoDB values for the number of iterations depending on mechanism
	scramSha1Iterations   = 10000
	scramSha256Iterations = 15000

	sha256mechanismName = "SCRAM-SHA-256"
	// MONGODB-CR is an umbrella term for SCRAM-SHA-1 and MONGODB-CR for legacy reasons, once MONGODB-CR
	// is enabled, users can auth with SCRAM-SHA-1 credentials
	sha1MechanismName = "MONGODB-CR"
)

type ScramCreds struct {
	IterationCount int    `json:"iterationCount"`
	Salt           string `json:"salt"`
	ServerKey      string `json:"serverKey"`
	StoredKey      string `json:"storedKey"`
}

func ComputeScramSha256Creds(username, password string, salt []byte) (ScramCreds, error) {
	return computeCreds(username, password, salt, sha256mechanismName)
}

func ComputeScramSha1Creds(username, password string, salt []byte) (ScramCreds, error) {
	return computeCreds(username, password, salt, sha1MechanismName)
}

// computeCreds takes a plain text password and a specified mechanism name and generates
// the ScramShaCreds which will be embedded into a MongoDBUser.
func computeCreds(username, password string, salt []byte, name string) (ScramCreds, error) {
	var hashConstructor func() hash.Hash
	iterations := 0
	if name == sha256mechanismName {
		hashConstructor = sha256.New
		iterations = scramSha256Iterations
	} else if name == sha1MechanismName {
		hashConstructor = sha1.New
		iterations = scramSha1Iterations

		// MONGODB-CR/SCRAM-SHA-1 requires the hash of the password being passed computeScramCredentials
		// instead of the plain text password.
		password = md5Hex(username + ":mongo:" + password)
	} else {
		return ScramCreds{}, fmt.Errorf("unrecognized SCRAM-SHA format %s", name)
	}
	base64EncodedSalt := base64.StdEncoding.EncodeToString(salt)
	return computeScramCredentials(hashConstructor, iterations, base64EncodedSalt, password)
}

func md5Hex(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func generateSaltedPassword(hashConstructor func() hash.Hash, password string, salt []byte, iterationCount int) ([]byte, error) {
	preparedPassword, err := stringprep.SASLprep.Prepare(password)
	if err != nil {
		return nil, fmt.Errorf("error SASLprep'ing password: %s", err)
	}

	result, err := hmacIteration(hashConstructor, []byte(preparedPassword), salt, iterationCount)
	if err != nil {
		return nil, fmt.Errorf("error running hmacIteration: %s", err)
	}
	return result, nil
}

func hmacIteration(hashConstructor func() hash.Hash, input, salt []byte, iterationCount int) ([]byte, error) {
	hashSize := hashConstructor().Size()

	// incorrect salt size will pass validation, but the credentials will be invalid. i.e. it will not
	// be possible to auth with the password provided to create the credentials.
	if len(salt) != hashSize-RFC5802MandatedSaltSize {
		return nil, fmt.Errorf("salt should have a size of %v bytes, but instead has a size of %v bytes", hashSize-RFC5802MandatedSaltSize, len(salt))
	}

	startKey := append(salt, 0, 0, 0, 1)
	result := make([]byte, hashSize)

	hmacHash := hmac.New(hashConstructor, input)
	if _, err := hmacHash.Write(startKey); err != nil {
		return nil, fmt.Errorf("error running hmacHash: %s", err)
	}

	intermediateDigest := hmacHash.Sum(nil)

	for i := 0; i < len(intermediateDigest); i++ {
		result[i] = intermediateDigest[i]
	}

	for i := 1; i < iterationCount; i++ {
		hmacHash.Reset()
		if _, err := hmacHash.Write(intermediateDigest); err != nil {
			return nil, fmt.Errorf("error running hmacHash: %s", err)
		}

		intermediateDigest = hmacHash.Sum(nil)

		for i := 0; i < len(intermediateDigest); i++ {
			result[i] ^= intermediateDigest[i]
		}
	}

	return result, nil
}

func generateClientOrServerKey(hashConstructor func() hash.Hash, saltedPassword []byte, input string) ([]byte, error) {
	hmacHash := hmac.New(hashConstructor, saltedPassword)
	if _, err := hmacHash.Write([]byte(input)); err != nil {
		return nil, fmt.Errorf("error running hmacHash: %s", err)
	}

	return hmacHash.Sum(nil), nil
}

func generateStoredKey(hashConstructor func() hash.Hash, clientKey []byte) ([]byte, error) {
	h := hashConstructor()
	if _, err := h.Write(clientKey); err != nil {
		return nil, fmt.Errorf("error hashing: %s", err)
	}
	return h.Sum(nil), nil
}

func generateSecrets(hashConstructor func() hash.Hash, password string, salt []byte, iterationCount int) (storedKey, serverKey []byte, err error) {
	saltedPassword, err := generateSaltedPassword(hashConstructor, password, salt, iterationCount)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating salted password: %s", err)
	}

	clientKey, err := generateClientOrServerKey(hashConstructor, saltedPassword, clientKeyInput)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating client key: %s", err)
	}

	storedKey, err = generateStoredKey(hashConstructor, clientKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating stored key: %s", err)
	}

	serverKey, err = generateClientOrServerKey(hashConstructor, saltedPassword, serverKeyInput)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating server key: %s", err)
	}

	return storedKey, serverKey, err
}

func generateB64EncodedSecrets(hashConstructor func() hash.Hash, password, b64EncodedSalt string, iterationCount int) (storedKey, serverKey string, err error) {
	salt, err := base64.StdEncoding.DecodeString(b64EncodedSalt)
	if err != nil {
		return "", "", fmt.Errorf("error decoding salt: %s", err)
	}

	unencodedStoredKey, unencodedServerKey, err := generateSecrets(hashConstructor, password, salt, iterationCount)
	if err != nil {
		return "", "", fmt.Errorf("error generating secrets: %s", err)
	}

	storedKey = base64.StdEncoding.EncodeToString(unencodedStoredKey)
	serverKey = base64.StdEncoding.EncodeToString(unencodedServerKey)
	return storedKey, serverKey, nil
}

// password should be encrypted in the case of SCRAM-SHA-1 and unencrypted in the case of SCRAM-SHA-256
func computeScramCredentials(hashConstructor func() hash.Hash, iterationCount int, base64EncodedSalt string, password string) (ScramCreds, error) {
	storedKey, serverKey, err := generateB64EncodedSecrets(hashConstructor, password, base64EncodedSalt, iterationCount)
	if err != nil {
		return ScramCreds{}, fmt.Errorf("error generating SCRAM-SHA keys: %s", err)
	}

	return ScramCreds{IterationCount: iterationCount, Salt: base64EncodedSalt, StoredKey: storedKey, ServerKey: serverKey}, nil
}
