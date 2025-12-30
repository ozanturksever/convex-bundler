package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ozanturksever/convex-bundler/pkg/bundle"
	"github.com/ozanturksever/convex-bundler/pkg/cli"
	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
	"github.com/ozanturksever/convex-bundler/pkg/predeploy"
	"github.com/ozanturksever/convex-bundler/pkg/version"
)

// TestIntegration_FullBundleWorkflow is the main integration test that validates
// the entire bundling workflow from CLI parsing to final bundle output.
// This test requires Docker. The backend binary is downloaded inside the container.
func TestIntegration_FullBundleWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	// Setup
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output-bundle")

	// Don't provide a backend binary - let the container download the appropriate Linux binary
	// This is necessary because the local binary may be for a different platform (e.g., macOS)
	// and cannot run inside the Linux Docker container.

	var err error

	// Test CLI parsing (with SkipValidation since we're not providing a backend binary)
	args := []string{
		"convex-bundler",
		"--app", "testdata/sample-app",
		"--output", outputDir,
		"--backend-binary", "/placeholder", // Placeholder for CLI parsing test
		"--name", "Test Backend",
		"--version", "2.0.0",
		"--platform", "linux-x64",
	}

	config, err := cli.Parse(args, cli.ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"testdata/sample-app"}, config.Apps)
	assert.Equal(t, outputDir, config.Output)
	assert.Equal(t, "/placeholder", config.BackendBinary)
	assert.Equal(t, "Test Backend", config.Name)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, "linux-x64", config.Platform)

	// Test version detection (should use CLI override)
	detectedVersion, err := version.Detect(config.Apps[0], config.Version)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", detectedVersion)

	// Test credential generation
	creds, err := credentials.Generate(config.Name)
	require.NoError(t, err)
	assert.NotEmpty(t, creds.AdminKey)
	assert.NotEmpty(t, creds.InstanceSecret)

	// Test manifest generation
	mf := manifest.New(manifest.Options{
		Name:     config.Name,
		Version:  detectedVersion,
		Apps:     config.Apps,
		Platform: config.Platform,
	})
	assert.Equal(t, "Test Backend", mf.Name)
	assert.Equal(t, "2.0.0", mf.Version)
	assert.Equal(t, "linux-x64", mf.Platform)
	assert.Len(t, mf.Apps, 1)

	// Test pre-deployment (this requires Docker)
	// Use empty BackendBinary to let the container download the appropriate Linux binary
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          config.Apps,
		BackendBinary: "", // Let container download the Linux binary
		OutputDir:     tmpDir,
		Platform:      config.Platform,
		DockerImage:   "node:20-slim", // Use base image for testing (will download deps)
	})
	require.NoError(t, err)
	assert.NotEmpty(t, predeployResult.DatabasePath)
	assert.DirExists(t, predeployResult.StoragePath)

	// Create a fake backend binary for the bundle step
	// In real usage, the user would provide the actual binary
	fakeBackendBinary := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackendBinary, []byte("#!/bin/bash\necho 'fake backend for bundle test'"), 0755)
	require.NoError(t, err)

	// Test bundle assembly
	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackendBinary,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify bundle structure
	assertBundleStructure(t, outputDir)

	// Verify manifest.json content
	assertManifestContent(t, outputDir, mf)

	// Verify credentials.json content
	assertCredentialsContent(t, outputDir)
}

// TestIntegration_VersionDetection_PackageJSON tests version detection from package.json
func TestIntegration_VersionDetection_PackageJSON(t *testing.T) {
	// Test with no CLI override - should read from package.json
	detectedVersion, err := version.Detect("testdata/sample-app", "")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", detectedVersion) // From testdata/sample-app/package.json
}

// TestIntegration_VersionDetection_CLIOverride tests that CLI version takes priority
func TestIntegration_VersionDetection_CLIOverride(t *testing.T) {
	detectedVersion, err := version.Detect("testdata/sample-app", "9.9.9")
	require.NoError(t, err)
	assert.Equal(t, "9.9.9", detectedVersion)
}

// TestIntegration_CLIParsing_RequiredFlags tests CLI validation for required flags
func TestIntegration_CLIParsing_RequiredFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing --app",
			args:    []string{"convex-bundler", "--output", "/tmp/out", "--backend-binary", "/bin/backend"},
			wantErr: "at least one --app is required",
		},
		{
			name:    "missing --output",
			args:    []string{"convex-bundler", "--app", "/app", "--backend-binary", "/bin/backend"},
			wantErr: "--output is required",
		},
		{
			name:    "missing --backend-binary",
			args:    []string{"convex-bundler", "--app", "/app", "--output", "/tmp/out"},
			wantErr: "--backend-binary is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cli.Parse(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestIntegration_CLIParsing_MultipleApps tests parsing multiple --app flags
func TestIntegration_CLIParsing_MultipleApps(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app1",
		"--app", "/app2",
		"--app", "/app3",
		"--output", "/tmp/out",
		"--backend-binary", "/bin/backend",
	}

	config, err := cli.Parse(args, cli.ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"/app1", "/app2", "/app3"}, config.Apps)
}

// TestIntegration_CLIParsing_Defaults tests default values
func TestIntegration_CLIParsing_Defaults(t *testing.T) {
	args := []string{
		"convex-bundler",
		"--app", "/app",
		"--output", "/tmp/out",
		"--backend-binary", "/bin/backend",
	}

	config, err := cli.Parse(args, cli.ParseOptions{SkipValidation: true})
	require.NoError(t, err)
	assert.Equal(t, "Convex Backend", config.Name)
	assert.Equal(t, "", config.Version) // No default, will be detected
	assert.Equal(t, "linux-x64", config.Platform)
}

// TestIntegration_ManifestSerialization tests manifest JSON serialization
func TestIntegration_ManifestSerialization(t *testing.T) {
	mf := manifest.New(manifest.Options{
		Name:     "My App",
		Version:  "1.0.0",
		Apps:     []string{"/app1", "/app2"},
		Platform: "linux-arm64",
	})

	data, err := mf.ToJSON()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "My App", parsed["name"])
	assert.Equal(t, "1.0.0", parsed["version"])
	assert.Equal(t, "linux-arm64", parsed["platform"])
	assert.NotEmpty(t, parsed["createdAt"])
	apps := parsed["apps"].([]interface{})
	assert.Len(t, apps, 2)
}

// TestIntegration_CredentialGeneration tests credential generation
func TestIntegration_CredentialGeneration(t *testing.T) {
	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	// Verify credentials are non-empty and have expected format
	assert.NotEmpty(t, creds.AdminKey)
	assert.NotEmpty(t, creds.InstanceSecret)
	// Instance secret should be a 64-character hex string
	assert.Len(t, creds.InstanceSecret, 64)

	// Test JSON serialization
	data, err := creds.ToJSON()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.NotEmpty(t, parsed["adminKey"])
	assert.NotEmpty(t, parsed["instanceSecret"])
}

// TestIntegration_BundleWithoutPredeploy tests bundle creation without pre-deployment
// (useful for testing bundle assembly in isolation)
func TestIntegration_BundleWithoutPredeploy(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")
	backendBinary := filepath.Join(tmpDir, "fake-backend")
	databasePath := filepath.Join(tmpDir, "convex.db")
	storagePath := filepath.Join(tmpDir, "storage")

	// Create fake files
	err := os.WriteFile(backendBinary, []byte("fake"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(databasePath, []byte("fake db"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(storagePath, 0755)
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	creds, err := credentials.Generate("Test")
	require.NoError(t, err)

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: backendBinary,
		DatabasePath:  databasePath,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	assertBundleStructure(t, outputDir)
}

// Helper functions

func assertBundleStructure(t *testing.T, outputDir string) {
	t.Helper()

	// Check backend binary exists and is executable
	backendPath := filepath.Join(outputDir, "backend")
	info, err := os.Stat(backendPath)
	require.NoError(t, err, "backend binary should exist")
	assert.True(t, info.Mode()&0111 != 0, "backend should be executable")

	// Check convex.db exists
	dbPath := filepath.Join(outputDir, "convex.db")
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "convex.db should exist")

	// Check storage directory exists
	storagePath := filepath.Join(outputDir, "storage")
	info, err = os.Stat(storagePath)
	require.NoError(t, err, "storage directory should exist")
	assert.True(t, info.IsDir(), "storage should be a directory")

	// Check manifest.json exists
	manifestPath := filepath.Join(outputDir, "manifest.json")
	_, err = os.Stat(manifestPath)
	require.NoError(t, err, "manifest.json should exist")

	// Check credentials.json exists
	credsPath := filepath.Join(outputDir, "credentials.json")
	_, err = os.Stat(credsPath)
	require.NoError(t, err, "credentials.json should exist")
}

func assertManifestContent(t *testing.T, outputDir string, expected *manifest.Manifest) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	require.NoError(t, err)

	var mf manifest.Manifest
	err = json.Unmarshal(data, &mf)
	require.NoError(t, err)

	assert.Equal(t, expected.Name, mf.Name)
	assert.Equal(t, expected.Version, mf.Version)
	assert.Equal(t, expected.Platform, mf.Platform)
	assert.Equal(t, expected.Apps, mf.Apps)
	assert.NotEmpty(t, mf.CreatedAt)
}

func assertCredentialsContent(t *testing.T, outputDir string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(outputDir, "credentials.json"))
	require.NoError(t, err)

	var creds map[string]string
	err = json.Unmarshal(data, &creds)
	require.NoError(t, err)

	assert.NotEmpty(t, creds["adminKey"])
	assert.NotEmpty(t, creds["instanceSecret"])
}
