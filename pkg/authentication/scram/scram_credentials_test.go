package scram

//
//func TestGetDeterministicSalt(t *testing.T) {
//	t.Run("same name and sha 256 hashing algorithm should result in same salt", func(t *testing.T) {
//		assert.Equal(t, getDeterministicSalt("my-mdb", sha256.New), getDeterministicSalt("my-mdb", sha256.New))
//	})
//
//	t.Run("same name and sha 1 hashing algorithm should result in same salt", func(t *testing.T) {
//		assert.Equal(t, getDeterministicSalt("my-mdb", sha1.New), getDeterministicSalt("my-mdb", sha1.New))
//	})
//
//	t.Run("different resource name results in different salt", func(t *testing.T) {
//		assert.NotEqual(t, getDeterministicSalt("my-different-mdb", sha256.New), getDeterministicSalt("my-mdb", sha256.New))
//	})
//
//	t.Run("using different algorithm results in different salt", func(t *testing.T) {
//		assert.NotEqual(t, getDeterministicSalt("my-mdb", sha256.New), getDeterministicSalt("my-mdb", sha1.New))
//	})
//
//	t.Run("salt is okay with short name", func(t *testing.T) {
//		assert.NotPanics(t, func() {
//			getDeterministicSalt("a", sha256.New)
//			getDeterministicSalt("a", sha1.New)
//		})
//	})
//}
//
//func TestComputeScram1AndScram256Credentials(t *testing.T) {
//	scram1Creds, scram2Creds, err := computeScram1AndScram256Credentials("my-resource", "my-username", "_BkuBebtnhE9")
//	assert.NoError(t, err)
//	t.Run("Subsequent generation results in the same credentials if password has not changed", func(t *testing.T) {
//		newScram1Creds, newScram2Creds, err := computeScram1AndScram256Credentials("my-resource", "my-username", "_BkuBebtnhE9")
//		assert.NoError(t, err)
//
//		assert.Equal(t, scram1Creds, newScram1Creds)
//		assert.Equal(t, scram2Creds, newScram2Creds)
//	})
//
//	t.Run("Subsequent generation results in different credentials if the password has changed", func(t *testing.T) {
//		newPassword := "k4J7wNttEbUJPigFnl"
//		newScram1Creds, newScram2Creds, err := computeScram1AndScram256Credentials("my-resource", "my-username", newPassword)
//		assert.NoError(t, err)
//
//		assert.NotEqual(t, scram1Creds, newScram1Creds)
//		assert.NotEqual(t, scram2Creds, newScram2Creds)
//	})
//}
