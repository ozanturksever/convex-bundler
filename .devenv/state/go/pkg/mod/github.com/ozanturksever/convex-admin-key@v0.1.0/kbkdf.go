// Package adminkey implements admin key generation for Convex backend instances.
package adminkey

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
)

// kbkdfCTRHMAC implements NIST SP 800-108r1 Key Derivation Function in Counter Mode
// using HMAC-SHA256 as the PRF, matching the aws-lc-rs implementation.
//
// The PRF input format is: Counter || Info
// Where:
//   - Counter is a 32-bit big-endian counter starting at 1
//   - Info is the fixed info string (label/purpose)
//
// This matches aws-lc-rs's simplified KBKDF implementation which does NOT include
// the separator byte or length field that the full SP 800-108 spec allows.
func kbkdfCTRHMAC(secret []byte, info []byte, outputLen int) []byte {
	h := hmac.New(sha256.New, secret)
	hashLen := h.Size() // 32 bytes for SHA256

	// Calculate number of iterations needed
	n := (outputLen + hashLen - 1) / hashLen

	result := make([]byte, 0, n*hashLen)

	for i := uint32(1); i <= uint32(n); i++ {
		h.Reset()

		// Counter (32-bit big-endian)
		counter := make([]byte, 4)
		binary.BigEndian.PutUint32(counter, i)
		h.Write(counter)

		// Info (fixed info string - the label/purpose)
		h.Write(info)

		result = append(result, h.Sum(nil)...)
	}

	return result[:outputLen]
}
