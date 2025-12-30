package adminkey

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const (
	// adminKeyVersion is the version byte for admin keys
	adminKeyVersion byte = 1

	// keyLen is the derived key length (AES-128)
	keyLen = 16

	// nonceLen is the nonce length for GCM-SIV
	nonceLen = 12

	// purposeAdminKey is the purpose string for admin key derivation
	purposeAdminKey = "admin key"
)

// Secret represents a 32-byte instance secret
type Secret [32]byte

// ParseSecret parses a hex-encoded secret string into a Secret
func ParseSecret(s string) (Secret, error) {
	var secret Secret
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return secret, fmt.Errorf("couldn't hex-decode secret: %w", err)
	}
	if len(decoded) != 32 {
		return secret, fmt.Errorf("hex-decoded secret was %d bytes, not 32", len(decoded))
	}
	copy(secret[:], decoded)
	return secret, nil
}

// GenerateSecret generates a new random 32-byte secret
func GenerateSecret() (Secret, error) {
	var secret Secret
	_, err := rand.Read(secret[:])
	if err != nil {
		return secret, fmt.Errorf("failed to generate random secret: %w", err)
	}
	return secret, nil
}

// String returns the hex-encoded secret
func (s Secret) String() string {
	return hex.EncodeToString(s[:])
}

// randomEncryptor encrypts data using AES-128-GCM-SIV with derived keys
type randomEncryptor struct {
	aead cipher.AEAD
}

// newRandomEncryptor creates a new encryptor with a key derived from the secret
func newRandomEncryptor(secret Secret, purpose string) (*randomEncryptor, error) {
	derivedKey := kbkdfCTRHMAC(secret[:], []byte(purpose), keyLen)

	// Create AES-GCM-SIV cipher
	aead, err := newAESGCMSIV(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD cipher: %w", err)
	}

	return &randomEncryptor{aead: aead}, nil
}

// encryptProto encrypts a protobuf message and returns hex-encoded ciphertext
// Format: version || nonce || ciphertext || tag (all hex-encoded)
func (e *randomEncryptor) encryptProto(version byte, message []byte) (string, error) {
	// Generate random nonce
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// AAD is just the version byte
	aad := []byte{version}

	// Encrypt (Seal appends ciphertext+tag to the first argument)
	ciphertext := e.aead.Seal(nil, nonce, message, aad)

	// Build output: version || nonce || ciphertext (includes tag)
	output := make([]byte, 0, 1+nonceLen+len(ciphertext))
	output = append(output, version)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	return hex.EncodeToString(output), nil
}
