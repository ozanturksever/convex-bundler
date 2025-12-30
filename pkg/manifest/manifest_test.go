package manifest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	mf := New(Options{
		Name:     "Test Backend",
		Version:  "1.0.0",
		Apps:     []string{"/app1", "/app2"},
		Platform: "linux-x64",
	})

	assert.Equal(t, "Test Backend", mf.Name)
	assert.Equal(t, "1.0.0", mf.Version)
	assert.Equal(t, []string{"/app1", "/app2"}, mf.Apps)
	assert.Equal(t, "linux-x64", mf.Platform)
	assert.NotEmpty(t, mf.CreatedAt)

	// Verify CreatedAt is a valid RFC3339 timestamp
	_, err := time.Parse(time.RFC3339, mf.CreatedAt)
	require.NoError(t, err)
}

func TestManifest_ToJSON(t *testing.T) {
	mf := New(Options{
		Name:     "My App",
		Version:  "2.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-arm64",
	})

	data, err := mf.ToJSON()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "My App", parsed["name"])
	assert.Equal(t, "2.0.0", parsed["version"])
	assert.Equal(t, "linux-arm64", parsed["platform"])
	assert.NotEmpty(t, parsed["createdAt"])

	apps := parsed["apps"].([]interface{})
	assert.Len(t, apps, 1)
	assert.Equal(t, "/app", apps[0])
}

func TestManifest_ToJSON_MultipleApps(t *testing.T) {
	mf := New(Options{
		Name:     "Multi App",
		Version:  "1.0.0",
		Apps:     []string{"/app1", "/app2", "/app3"},
		Platform: "linux-x64",
	})

	data, err := mf.ToJSON()
	require.NoError(t, err)

	var parsed Manifest
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, 3, len(parsed.Apps))
	assert.Equal(t, "/app1", parsed.Apps[0])
	assert.Equal(t, "/app2", parsed.Apps[1])
	assert.Equal(t, "/app3", parsed.Apps[2])
}

func TestManifest_ToJSON_Formatting(t *testing.T) {
	mf := New(Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	data, err := mf.ToJSON()
	require.NoError(t, err)

	// Should be indented (pretty printed)
	assert.Contains(t, string(data), "\n")
	assert.Contains(t, string(data), "  ")
}
