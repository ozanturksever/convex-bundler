package adminkey

import (
	"strings"
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Secret should be 32 bytes, which is 64 hex characters
	hexStr := secret.String()
	if len(hexStr) != 64 {
		t.Errorf("Secret hex string length = %d, want 64", len(hexStr))
	}
}

func TestParseSecret(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid secret",
			input:   "4361726e697461732c206c69746572616c6c79206d65616e696e6720226c6974",
			wantErr: false,
		},
		{
			name:    "too short",
			input:   "4361726e697461732c206c69746572616c6c79206d65616e696e67",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			input:   "zzzz726e697461732c206c69746572616c6c79206d65616e696e6720226c6974",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSecret(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIssueAdminKey(t *testing.T) {
	// Use the dev instance secret from convex-backend
	secretStr := "4361726e697461732c206c69746572616c6c79206d65616e696e6720226c6974"
	secret, err := ParseSecret(secretStr)
	if err != nil {
		t.Fatalf("ParseSecret() error = %v", err)
	}

	key, err := IssueAdminKey(secret, "carnitas", 0, false)
	if err != nil {
		t.Fatalf("IssueAdminKey() error = %v", err)
	}

	// Key should be in format "instance_name|encrypted_part"
	if !strings.HasPrefix(key, "carnitas|") {
		t.Errorf("Key should start with 'carnitas|', got %s", key)
	}

	// The encrypted part should be hex-encoded
	parts := strings.Split(key, "|")
	if len(parts) != 2 {
		t.Fatalf("Key should have exactly one '|' separator, got %d parts", len(parts))
	}

	encryptedPart := parts[1]
	// Version (1 byte) + nonce (12 bytes) + min ciphertext + tag (16 bytes) = at least 29 bytes = 58 hex chars
	if len(encryptedPart) < 58 {
		t.Errorf("Encrypted part too short: %d chars", len(encryptedPart))
	}
}

func TestIssueAdminKeyReadOnly(t *testing.T) {
	secretStr := "4361726e697461732c206c69746572616c6c79206d65616e696e6720226c6974"
	secret, err := ParseSecret(secretStr)
	if err != nil {
		t.Fatalf("ParseSecret() error = %v", err)
	}

	key, err := IssueAdminKey(secret, "carnitas", 0, true)
	if err != nil {
		t.Fatalf("IssueAdminKey() error = %v", err)
	}

	if !strings.HasPrefix(key, "carnitas|") {
		t.Errorf("Key should start with 'carnitas|', got %s", key)
	}
}

func TestIssueAdminKeyWithMemberID(t *testing.T) {
	secretStr := "4361726e697461732c206c69746572616c6c79206d65616e696e6720226c6974"
	secret, err := ParseSecret(secretStr)
	if err != nil {
		t.Fatalf("ParseSecret() error = %v", err)
	}

	key, err := IssueAdminKey(secret, "carnitas", 42, false)
	if err != nil {
		t.Fatalf("IssueAdminKey() error = %v", err)
	}

	if !strings.HasPrefix(key, "carnitas|") {
		t.Errorf("Key should start with 'carnitas|', got %s", key)
	}
}

func TestIssueSystemKey(t *testing.T) {
	secretStr := "4361726e697461732c206c69746572616c6c79206d65616e696e6720226c6974"
	secret, err := ParseSecret(secretStr)
	if err != nil {
		t.Fatalf("ParseSecret() error = %v", err)
	}

	key, err := IssueSystemKey(secret, "carnitas")
	if err != nil {
		t.Fatalf("IssueSystemKey() error = %v", err)
	}

	if !strings.HasPrefix(key, "carnitas|") {
		t.Errorf("Key should start with 'carnitas|', got %s", key)
	}
}

func TestKBKDF(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}

	derived := kbkdfCTRHMAC(secret, []byte("admin key"), 16)
	if len(derived) != 16 {
		t.Errorf("Derived key length = %d, want 16", len(derived))
	}

	// Derived key should be deterministic
	derived2 := kbkdfCTRHMAC(secret, []byte("admin key"), 16)
	for i := range derived {
		if derived[i] != derived2[i] {
			t.Error("KBKDF should produce deterministic output")
			break
		}
	}

	// Different purpose should produce different key
	derived3 := kbkdfCTRHMAC(secret, []byte("different purpose"), 16)
	same := true
	for i := range derived {
		if derived[i] != derived3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Different purposes should produce different keys")
	}
}

func TestAdminKeyProtoEncode(t *testing.T) {
	proto := &adminKeyProto{
		instanceName: nil,
		issuedS:      1234567890,
		identityType: adminIdentityMember,
		memberID:     42,
		isReadOnly:   false,
	}

	encoded := proto.encode()
	if len(encoded) == 0 {
		t.Error("Encoded protobuf should not be empty")
	}

	// Test with system identity
	proto2 := &adminKeyProto{
		instanceName: nil,
		issuedS:      1234567890,
		identityType: adminIdentitySystem,
		memberID:     0,
		isReadOnly:   false,
	}

	encoded2 := proto2.encode()
	if len(encoded2) == 0 {
		t.Error("Encoded protobuf should not be empty")
	}
}
