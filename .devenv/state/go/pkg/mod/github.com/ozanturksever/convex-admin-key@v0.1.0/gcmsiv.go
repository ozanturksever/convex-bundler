package adminkey

import (
	"crypto/cipher"

	siv "github.com/secure-io/siv-go"
)

// newAESGCMSIV creates an AES-GCM-SIV AEAD cipher (RFC 8452).
// This provides nonce-misuse resistance, matching the Rust implementation.
func newAESGCMSIV(key []byte) (cipher.AEAD, error) {
	return siv.NewGCM(key)
}
