package version

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetect_CLIOverride(t *testing.T) {
	version, err := Detect("/nonexistent", "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version)
}

func TestDetect_PackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	packageJSON := `{"name": "test", "version": "2.3.4"}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	version, err := Detect(tmpDir, "")
	require.NoError(t, err)
	assert.Equal(t, "2.3.4", version)
}

func TestDetect_Default(t *testing.T) {
	tmpDir := t.TempDir()
	// No package.json, no git tags

	version, err := Detect(tmpDir, "")
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", version)
}

func TestDetect_CLIOverrideTakesPriority(t *testing.T) {
	tmpDir := t.TempDir()
	packageJSON := `{"name": "test", "version": "2.3.4"}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// CLI override should take priority over package.json
	version, err := Detect(tmpDir, "9.9.9")
	require.NoError(t, err)
	assert.Equal(t, "9.9.9", version)
}

func TestDetect_InvalidPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Should fall back to default
	version, err := Detect(tmpDir, "")
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", version)
}

func TestDetect_PackageJSONWithoutVersion(t *testing.T) {
	tmpDir := t.TempDir()
	packageJSON := `{"name": "test"}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Should fall back to default when version field is missing
	version, err := Detect(tmpDir, "")
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", version)
}

func TestDetectFromPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	packageJSON := `{
		"name": "my-app",
		"version": "1.0.0",
		"description": "Test app"
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	version, err := detectFromPackageJSON(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", version)
}

func TestDetectFromPackageJSON_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := detectFromPackageJSON(tmpDir)
	require.Error(t, err)
}
