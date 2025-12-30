package credentials

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	creds, err := Generate("test-instance")
	require.NoError(t, err)

	assert.NotEmpty(t, creds.AdminKey)
	assert.NotEmpty(t, creds.InstanceSecret)

	// Keys should be different
	assert.NotEqual(t, creds.AdminKey, creds.InstanceSecret)
}

func TestGenerate_Uniqueness(t *testing.T) {
	creds1, err := Generate("test-instance-1")
	require.NoError(t, err)

	creds2, err := Generate("test-instance-2")
	require.NoError(t, err)

	// Each generation should produce unique keys
	assert.NotEqual(t, creds1.AdminKey, creds2.AdminKey)
	assert.NotEqual(t, creds1.InstanceSecret, creds2.InstanceSecret)
}

func TestGenerate_KeyLength(t *testing.T) {
	creds, err := Generate("test-instance")
	require.NoError(t, err)

	// Admin key from convex-admin-key library should be substantial
	assert.GreaterOrEqual(t, len(creds.AdminKey), 20)
	// Instance secret is 64-character hex (32 bytes)
	assert.Equal(t, 64, len(creds.InstanceSecret))
}

func TestGenerate_InstanceSecretFormat(t *testing.T) {
	creds, err := Generate("test-instance")
	require.NoError(t, err)

	// Instance secret should be a valid hex string (64 chars for 32 bytes)
	assert.Regexp(t, "^[0-9a-f]{64}$", creds.InstanceSecret)
}

func TestCredentials_ToJSON(t *testing.T) {
	creds, err := Generate("test-instance")
	require.NoError(t, err)

	data, err := creds.ToJSON()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, creds.AdminKey, parsed["adminKey"])
	assert.Equal(t, creds.InstanceSecret, parsed["instanceSecret"])
}

func TestCredentials_ToJSON_Formatting(t *testing.T) {
	creds, err := Generate("test-instance")
	require.NoError(t, err)

	data, err := creds.ToJSON()
	require.NoError(t, err)

	// Should be indented
	assert.Contains(t, string(data), "\n")
	assert.Contains(t, string(data), "  ")
}
