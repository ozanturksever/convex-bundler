package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ozanturksever/convex-bundler/pkg/cli"
	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
	"github.com/ozanturksever/convex-bundler/pkg/selfhost"
)

// TestIntegration_SelfHostFullWorkflow tests the complete selfhost workflow:
// create bundle -> create selfhost executable -> verify -> extract -> compare
func TestIntegration_SelfHostFullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Create a mock bundle directory with all required files
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createSelfHostTestBundle(t, bundleDir)

	// Step 2: Create a mock ops binary
	opsBinary := filepath.Join(tmpDir, "convex-backend-ops")
	createSelfHostMockOpsBinary(t, opsBinary)

	// Step 3: Create self-extracting executable
	selfhostPath := filepath.Join(tmpDir, "my-backend-selfhost")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinary,
		OutputPath:  selfhostPath,
		Platform:    "linux-x64",
		Compression: selfhost.CompressionGzip,
		OpsVersion:  "1.0.0",
	})
	require.NoError(t, err)

	// Step 4: Verify the selfhost executable was created
	info, err := os.Stat(selfhostPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "selfhost executable should be executable")

	// Step 5: Detect selfhost mode
	detectResult, err := selfhost.DetectSelfHostModeFromFile(selfhostPath)
	require.NoError(t, err)
	assert.True(t, detectResult.IsSelfHost, "should detect as selfhost executable")
	assert.Greater(t, detectResult.Offset, int64(0), "offset should be positive")

	// Step 6: Read header and verify metadata
	header, err := selfhost.ReadHeaderFromExecutable(selfhostPath)
	require.NoError(t, err)
	assert.Equal(t, selfhost.HeaderVersion, header.Version)
	assert.Equal(t, selfhost.HeaderFormat, header.Format)
	assert.Equal(t, selfhost.CompressionGzip, header.Compression)
	assert.Equal(t, "1.0.0", header.OpsVersion)
	assert.Equal(t, "Test Backend", header.Manifest.Name)
	assert.Equal(t, "1.0.0", header.Manifest.Version)
	assert.Equal(t, "linux-x64", header.Manifest.Platform)
	assert.NotEmpty(t, header.BundleChecksum)
	assert.Greater(t, header.BundleSize, int64(0))

	// Step 7: Verify integrity
	verifyResult, err := selfhost.Verify(selfhostPath)
	require.NoError(t, err)
	assert.True(t, verifyResult.Valid, "checksum verification should pass")
	assert.Equal(t, verifyResult.ExpectedChecksum, verifyResult.ActualChecksum)

	// Step 8: Extract bundle
	extractDir := filepath.Join(tmpDir, "extracted")
	extractedHeader, err := selfhost.Extract(selfhost.ExtractOptions{
		ExecutablePath: selfhostPath,
		OutputDir:      extractDir,
	})
	require.NoError(t, err)
	assert.Equal(t, header.Manifest.Name, extractedHeader.Manifest.Name)

	// Step 9: Verify extracted bundle structure
	assertSelfHostBundleStructure(t, extractDir)

	// Step 10: Compare extracted contents with original
	compareDirectories(t, bundleDir, extractDir)
}

// TestIntegration_SelfHostCLIParsing tests CLI argument parsing for selfhost command
func TestIntegration_SelfHostCLIParsing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid paths for testing
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	opsBinary := filepath.Join(tmpDir, "ops")
	require.NoError(t, os.WriteFile(opsBinary, []byte("mock"), 0755))

	outputPath := filepath.Join(tmpDir, "output")

	// Test IsSelfHostCommand detection
	assert.True(t, cli.IsSelfHostCommand([]string{"convex-bundler", "selfhost", "--bundle", bundleDir}))
	assert.False(t, cli.IsSelfHostCommand([]string{"convex-bundler", "--app", "/app"}))

	// Test ParseSelfHost
	args := []string{
		"selfhost",
		"--bundle", bundleDir,
		"--ops-binary", opsBinary,
		"--output", outputPath,
		"--platform", "linux-x64",
		"--compression", "gzip",
		"--ops-version", "2.0.0",
	}

	config, err := cli.ParseSelfHost(args)
	require.NoError(t, err)
	assert.Equal(t, bundleDir, config.BundleDir)
	assert.Equal(t, opsBinary, config.OpsBinary)
	assert.Equal(t, outputPath, config.Output)
	assert.Equal(t, "linux-x64", config.Platform)
	assert.Equal(t, "gzip", config.Compression)
	assert.Equal(t, "2.0.0", config.OpsVersion)
}

// TestIntegration_SelfHostCorruptedExecutable tests that corrupted executables fail verification
func TestIntegration_SelfHostCorruptedExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bundle and selfhost executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createSelfHostTestBundle(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createSelfHostMockOpsBinary(t, opsBinary)

	selfhostPath := filepath.Join(tmpDir, "selfhost")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: selfhostPath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Verify it's valid first
	verifyResult, err := selfhost.Verify(selfhostPath)
	require.NoError(t, err)
	assert.True(t, verifyResult.Valid)

	// Corrupt the executable
	data, err := os.ReadFile(selfhostPath)
	require.NoError(t, err)

	// Corrupt bytes in the middle (after header, in compressed data)
	corruptionOffset := len(data) / 2
	data[corruptionOffset] ^= 0xFF
	data[corruptionOffset+1] ^= 0xFF
	data[corruptionOffset+2] ^= 0xFF

	err = os.WriteFile(selfhostPath, data, 0755)
	require.NoError(t, err)

	// Verification should now fail
	verifyResult, err = selfhost.Verify(selfhostPath)
	require.NoError(t, err)
	assert.False(t, verifyResult.Valid, "corrupted executable should fail verification")
	assert.NotEqual(t, verifyResult.ExpectedChecksum, verifyResult.ActualChecksum)
}

// TestIntegration_SelfHostExtractWithSkipVerify tests extraction with checksum skip
func TestIntegration_SelfHostExtractWithSkipVerify(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bundle and selfhost executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createSelfHostTestBundle(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createSelfHostMockOpsBinary(t, opsBinary)

	selfhostPath := filepath.Join(tmpDir, "selfhost")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: selfhostPath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Extract with SkipVerify
	extractDir := filepath.Join(tmpDir, "extracted")
	header, err := selfhost.Extract(selfhost.ExtractOptions{
		ExecutablePath: selfhostPath,
		OutputDir:      extractDir,
		SkipVerify:     true,
	})
	require.NoError(t, err)
	assert.NotNil(t, header)

	// Verify extraction succeeded
	assertSelfHostBundleStructure(t, extractDir)
}

// TestIntegration_SelfHostMultipleApps tests bundle with multiple apps in manifest
func TestIntegration_SelfHostMultipleApps(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bundle with multiple apps
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	mf := manifest.New(manifest.Options{
		Name:     "Multi-App Backend",
		Version:  "2.0.0",
		Apps:     []string{"./app1", "./app2", "./app3"},
		Platform: "linux-arm64",
	})
	manifestData, err := mf.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), manifestData, 0644))

	// Create other required files
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "backend"), []byte("mock backend"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "convex.db"), []byte("mock db"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "credentials.json"), []byte(`{"adminKey":"key","instanceSecret":"secret"}`), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "storage"), 0755))

	opsBinary := filepath.Join(tmpDir, "ops")
	createSelfHostMockOpsBinary(t, opsBinary)

	selfhostPath := filepath.Join(tmpDir, "selfhost")
	err = selfhost.Create(selfhost.CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: selfhostPath,
		Platform:   "linux-arm64",
		OpsVersion: "1.5.0",
	})
	require.NoError(t, err)

	// Read header and verify apps
	header, err := selfhost.ReadHeaderFromExecutable(selfhostPath)
	require.NoError(t, err)
	assert.Equal(t, "Multi-App Backend", header.Manifest.Name)
	assert.Equal(t, "2.0.0", header.Manifest.Version)
	assert.Equal(t, "linux-arm64", header.Manifest.Platform)
	assert.Len(t, header.Manifest.Apps, 3)
	assert.Equal(t, []string{"./app1", "./app2", "./app3"}, header.Manifest.Apps)
	assert.Equal(t, "1.5.0", header.OpsVersion)
}

// TestIntegration_SelfHostNonSelfHostFile tests error handling for non-selfhost files
func TestIntegration_SelfHostNonSelfHostFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file (not selfhost)
	regularFile := filepath.Join(tmpDir, "regular")
	require.NoError(t, os.WriteFile(regularFile, []byte("just a regular file"), 0644))

	// Detect should return false
	result, err := selfhost.DetectSelfHostModeFromFile(regularFile)
	require.NoError(t, err)
	assert.False(t, result.IsSelfHost)

	// ReadHeader should fail
	_, err = selfhost.ReadHeaderFromExecutable(regularFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a self-host executable")

	// Verify should fail
	_, err = selfhost.Verify(regularFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain an embedded bundle")

	// Extract should fail
	_, err = selfhost.Extract(selfhost.ExtractOptions{
		ExecutablePath: regularFile,
		OutputDir:      filepath.Join(tmpDir, "extract"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain an embedded bundle")
}

// TestIntegration_SelfHostPlatformCompatibility tests platform compatibility checking
func TestIntegration_SelfHostPlatformCompatibility(t *testing.T) {
	// Test matching platform (should succeed)
	err := selfhost.CheckPlatformCompatibility(getExpectedPlatform())
	assert.NoError(t, err)

	// Test mismatching platform (should fail)
	err = selfhost.CheckPlatformCompatibility("nonexistent-platform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform mismatch")
}

// TestIntegration_SelfHostNestedStorageFiles tests that nested files in storage are preserved
func TestIntegration_SelfHostNestedStorageFiles(t *testing.T) {
	tmpDir := t.TempDir()

	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createSelfHostTestBundle(t, bundleDir)

	// Add nested files to storage
	nestedDir := filepath.Join(bundleDir, "storage", "nested", "deep")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "file1.txt"), []byte("nested content 1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "file2.txt"), []byte("nested content 2"), 0644))

	opsBinary := filepath.Join(tmpDir, "ops")
	createSelfHostMockOpsBinary(t, opsBinary)

	selfhostPath := filepath.Join(tmpDir, "selfhost")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: selfhostPath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Extract
	extractDir := filepath.Join(tmpDir, "extracted")
	_, err = selfhost.Extract(selfhost.ExtractOptions{
		ExecutablePath: selfhostPath,
		OutputDir:      extractDir,
	})
	require.NoError(t, err)

	// Verify nested files exist
	content1, err := os.ReadFile(filepath.Join(extractDir, "storage", "nested", "deep", "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content 1", string(content1))

	content2, err := os.ReadFile(filepath.Join(extractDir, "storage", "nested", "deep", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content 2", string(content2))
}

// Helper functions

// createSelfHostTestBundle creates a complete test bundle with all required files
func createSelfHostTestBundle(t *testing.T, bundleDir string) {
	t.Helper()

	// Create manifest.json
	mf := manifest.New(manifest.Options{
		Name:     "Test Backend",
		Version:  "1.0.0",
		Apps:     []string{"./app1"},
		Platform: "linux-x64",
	})
	manifestData, err := mf.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), manifestData, 0644))

	// Create mock backend binary
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "backend"), []byte("#!/bin/bash\necho 'mock backend'\n"), 0755))

	// Create mock database
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "convex.db"), []byte("SQLite format 3\x00mock database content"), 0644))

	// Create credentials
	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)
	credsData, err := creds.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "credentials.json"), credsData, 0644))

	// Create storage directory with a test file
	storageDir := filepath.Join(bundleDir, "storage")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(storageDir, "test-file.txt"), []byte("test storage content"), 0644))
}

// createSelfHostMockOpsBinary creates a mock convex-backend-ops binary
func createSelfHostMockOpsBinary(t *testing.T, path string) {
	t.Helper()
	content := `#!/bin/bash
# Mock convex-backend-ops binary
echo "convex-backend-ops mock"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0755))
}

// assertSelfHostBundleStructure verifies the extracted bundle has all required files
func assertSelfHostBundleStructure(t *testing.T, dir string) {
	t.Helper()

	// Check backend exists and is executable
	backendPath := filepath.Join(dir, "backend")
	info, err := os.Stat(backendPath)
	require.NoError(t, err, "backend should exist")
	assert.True(t, info.Mode()&0111 != 0, "backend should be executable")

	// Check convex.db exists
	_, err = os.Stat(filepath.Join(dir, "convex.db"))
	require.NoError(t, err, "convex.db should exist")

	// Check manifest.json exists and is valid JSON
	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err, "manifest.json should exist")
	var mf manifest.Manifest
	require.NoError(t, json.Unmarshal(manifestData, &mf), "manifest.json should be valid JSON")

	// Check credentials.json exists
	_, err = os.Stat(filepath.Join(dir, "credentials.json"))
	require.NoError(t, err, "credentials.json should exist")

	// Check storage directory exists
	info, err = os.Stat(filepath.Join(dir, "storage"))
	require.NoError(t, err, "storage directory should exist")
	assert.True(t, info.IsDir(), "storage should be a directory")
}

// compareDirectories compares two directories recursively
func compareDirectories(t *testing.T, originalDir, extractedDir string) {
	t.Helper()

	err := filepath.Walk(originalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(originalDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		extractedPath := filepath.Join(extractedDir, relPath)

		if info.IsDir() {
			extractedInfo, err := os.Stat(extractedPath)
			require.NoError(t, err, "directory should exist: %s", relPath)
			assert.True(t, extractedInfo.IsDir(), "should be a directory: %s", relPath)
		} else {
			originalContent, err := os.ReadFile(path)
			require.NoError(t, err)

			extractedContent, err := os.ReadFile(extractedPath)
			require.NoError(t, err, "file should exist: %s", relPath)

			assert.Equal(t, originalContent, extractedContent, "content should match for: %s", relPath)
		}

		return nil
	})
	require.NoError(t, err)
}

// getExpectedPlatform returns the expected platform string for the current runtime
func getExpectedPlatform() string {
	// This should match the logic in selfhost.getHostPlatform()
	platformMap := map[string]string{
		"linux-amd64":  "linux-x64",
		"linux-arm64":  "linux-arm64",
		"darwin-amd64": "darwin-x64",
		"darwin-arm64": "darwin-arm64",
	}
	key := runtime.GOOS + "-" + runtime.GOARCH
	if platform, ok := platformMap[key]; ok {
		return platform
	}
	return key
}
