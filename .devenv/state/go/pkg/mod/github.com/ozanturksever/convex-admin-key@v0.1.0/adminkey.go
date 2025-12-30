package adminkey

import (
	"fmt"
	"time"
)

// IssueAdminKey generates an admin key for the given instance and member ID.
//
// Parameters:
//   - secret: The 32-byte instance secret
//   - instanceName: Name of the Convex instance (e.g., "carnitas")
//   - memberID: Member ID for the admin key (use 0 for generic admin keys)
//   - isReadOnly: If true, creates a read-only key that can only run queries
//
// Returns the admin key in the format "instance_name|encrypted_part".
func IssueAdminKey(secret Secret, instanceName string, memberID uint64, isReadOnly bool) (string, error) {
	encryptor, err := newRandomEncryptor(secret, purposeAdminKey)
	if err != nil {
		return "", fmt.Errorf("failed to create encryptor: %w", err)
	}

	proto := &adminKeyProto{
		instanceName: nil, // Not included in new format
		issuedS:      uint64(time.Now().Unix()),
		identityType: adminIdentityMember,
		memberID:     memberID,
		isReadOnly:   isReadOnly,
	}

	encoded := proto.encode()
	encryptedPart, err := encryptor.encryptProto(adminKeyVersion, encoded)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt admin key: %w", err)
	}

	return formatAdminKey(instanceName, encryptedPart), nil
}

// IssueSystemKey generates a system key for the given instance.
// System keys are used for internal Convex operations.
//
// Parameters:
//   - secret: The 32-byte instance secret
//   - instanceName: Name of the Convex instance
//
// Returns the system key in the format "instance_name|encrypted_part".
func IssueSystemKey(secret Secret, instanceName string) (string, error) {
	encryptor, err := newRandomEncryptor(secret, purposeAdminKey)
	if err != nil {
		return "", fmt.Errorf("failed to create encryptor: %w", err)
	}

	proto := &adminKeyProto{
		instanceName: nil,
		issuedS:      uint64(time.Now().Unix()),
		identityType: adminIdentitySystem,
		memberID:     0,
		isReadOnly:   false,
	}

	encoded := proto.encode()
	encryptedPart, err := encryptor.encryptProto(adminKeyVersion, encoded)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt system key: %w", err)
	}

	return formatAdminKey(instanceName, encryptedPart), nil
}

// formatAdminKey formats an admin key as "instance_name|encrypted_part"
func formatAdminKey(instanceName, encryptedPart string) string {
	return instanceName + "|" + encryptedPart
}
