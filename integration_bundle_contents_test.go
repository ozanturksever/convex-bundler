package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/ozanturksever/convex-bundler/pkg/bundle"
	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
	"github.com/ozanturksever/convex-bundler/pkg/predeploy"
)

// TestIntegration_BundleContents_AllRequiredFiles verifies that a complete bundle
// contains all required files: backend, convex.db, storage/, manifest.json, credentials.json
func TestIntegration_BundleContents_AllRequiredFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Run predeploy to create database and storage
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          []string{"testdata/sample-app"},
		BackendBinary: "", // Let container download the backend
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim", // Use base image for testing
	})
	require.NoError(t, err, "predeploy should succeed")

	// Create a fake backend binary for bundle assembly
	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackend, []byte("#!/bin/bash\necho 'fake backend'"), 0755)
	require.NoError(t, err)

	// Generate credentials
	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	// Create manifest
	mf := manifest.New(manifest.Options{
		Name:     "Bundle Contents Test",
		Version:  "1.0.0",
		Apps:     []string{"testdata/sample-app"},
		Platform: "linux-x64",
	})

	// Create the bundle
	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err, "bundle creation should succeed")

	// Verify all required files exist
	t.Run("backend_exists_and_executable", func(t *testing.T) {
		backendPath := filepath.Join(outputDir, "backend")
		info, err := os.Stat(backendPath)
		require.NoError(t, err, "backend binary should exist")
		assert.True(t, info.Mode()&0111 != 0, "backend should be executable")
	})

	t.Run("convex_db_exists", func(t *testing.T) {
		dbPath := filepath.Join(outputDir, "convex.db")
		info, err := os.Stat(dbPath)
		require.NoError(t, err, "convex.db should exist")
		assert.True(t, info.Size() > 0, "convex.db should not be empty")
	})

	t.Run("storage_directory_exists", func(t *testing.T) {
		storagePath := filepath.Join(outputDir, "storage")
		info, err := os.Stat(storagePath)
		require.NoError(t, err, "storage directory should exist")
		assert.True(t, info.IsDir(), "storage should be a directory")
	})

	t.Run("manifest_json_exists", func(t *testing.T) {
		manifestPath := filepath.Join(outputDir, "manifest.json")
		_, err := os.Stat(manifestPath)
		require.NoError(t, err, "manifest.json should exist")
	})

	t.Run("credentials_json_exists", func(t *testing.T) {
		credsPath := filepath.Join(outputDir, "credentials.json")
		_, err := os.Stat(credsPath)
		require.NoError(t, err, "credentials.json should exist")
	})
}

// TestIntegration_BundleContents_ValidSQLiteDatabase verifies the database in the bundle
// is a valid SQLite database with tables created by Convex deployment
func TestIntegration_BundleContents_ValidSQLiteDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Run predeploy
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          []string{"testdata/sample-app"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim",
	})
	require.NoError(t, err)

	// Create fake backend for bundle
	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackend, []byte("fake"), 0755)
	require.NoError(t, err)

	creds, err := credentials.Generate("test")
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "DB Test",
		Version:  "1.0.0",
		Apps:     []string{"testdata/sample-app"},
		Platform: "linux-x64",
	})

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Open and validate the database
	dbPath := filepath.Join(outputDir, "convex.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err, "should be able to open database as SQLite")
	defer db.Close()

	// Verify the connection works
	err = db.Ping()
	require.NoError(t, err, "database should respond to ping")

	// Query for tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	require.NoError(t, err, "should be able to query sqlite_master")
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err)
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())

	// Database should have tables after Convex deployment
	assert.NotEmpty(t, tables, "database should contain tables after deployment")
	t.Logf("Database contains %d tables: %v", len(tables), tables)
}

// TestIntegration_BundleContents_ManifestContent verifies manifest.json contains correct data
func TestIntegration_BundleContents_ManifestContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Run predeploy
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          []string{"testdata/sample-app"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-arm64", // Test with a different platform
		DockerImage:   "node:20-slim",
	})
	require.NoError(t, err)

	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackend, []byte("fake"), 0755)
	require.NoError(t, err)

	creds, err := credentials.Generate("Manifest Test Instance")
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "My Custom App",
		Version:  "2.5.0",
		Apps:     []string{"testdata/sample-app"},
		Platform: "linux-arm64",
	})

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Read and parse manifest
	manifestData, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	require.NoError(t, err)

	var parsedManifest map[string]interface{}
	err = json.Unmarshal(manifestData, &parsedManifest)
	require.NoError(t, err)

	// Verify manifest contents
	assert.Equal(t, "My Custom App", parsedManifest["name"])
	assert.Equal(t, "2.5.0", parsedManifest["version"])
	assert.Equal(t, "linux-arm64", parsedManifest["platform"])
	assert.NotEmpty(t, parsedManifest["createdAt"], "manifest should have createdAt timestamp")

	apps, ok := parsedManifest["apps"].([]interface{})
	require.True(t, ok, "apps should be an array")
	assert.Len(t, apps, 1)
}

// TestIntegration_BundleContents_CredentialsContent verifies credentials.json contains
// valid adminKey and instanceSecret
func TestIntegration_BundleContents_CredentialsContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Run predeploy
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          []string{"testdata/sample-app"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim",
	})
	require.NoError(t, err)

	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackend, []byte("fake"), 0755)
	require.NoError(t, err)

	// Generate credentials with a specific instance name
	creds, err := credentials.Generate("my-test-instance")
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Creds Test",
		Version:  "1.0.0",
		Apps:     []string{"testdata/sample-app"},
		Platform: "linux-x64",
	})

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Read and parse credentials
	credsData, err := os.ReadFile(filepath.Join(outputDir, "credentials.json"))
	require.NoError(t, err)

	var parsedCreds map[string]string
	err = json.Unmarshal(credsData, &parsedCreds)
	require.NoError(t, err)

	// Verify credentials
	adminKey := parsedCreds["adminKey"]
	instanceSecret := parsedCreds["instanceSecret"]

	assert.NotEmpty(t, adminKey, "adminKey should not be empty")
	assert.NotEmpty(t, instanceSecret, "instanceSecret should not be empty")

	// Instance secret should be 64-character hex string (32 bytes)
	assert.Len(t, instanceSecret, 64, "instanceSecret should be 64 hex characters")

	// Admin key should contain the instance name (format: instanceName|base64data)
	assert.Contains(t, adminKey, "|", "adminKey should contain pipe separator")

	t.Logf("Admin key prefix: %s...", adminKey[:min(30, len(adminKey))])
	t.Logf("Instance secret prefix: %s...", instanceSecret[:min(16, len(instanceSecret))])
}

// TestIntegration_BundleContents_StorageDirectory verifies the storage directory
// is properly included in the bundle
func TestIntegration_BundleContents_StorageDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires Docker)")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Run predeploy
	predeployResult, err := predeploy.Run(predeploy.Options{
		Apps:          []string{"testdata/sample-app"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim",
	})
	require.NoError(t, err)

	// Verify predeploy created storage path
	assert.DirExists(t, predeployResult.StoragePath, "predeploy should create storage directory")

	// Log what's in the storage directory from predeploy
	entries, _ := os.ReadDir(predeployResult.StoragePath)
	t.Logf("Predeploy storage contains %d entries", len(entries))
	for _, entry := range entries {
		t.Logf("  - %s (dir=%v)", entry.Name(), entry.IsDir())
	}

	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err = os.WriteFile(fakeBackend, []byte("fake"), 0755)
	require.NoError(t, err)

	creds, err := credentials.Generate("storage-test")
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Storage Test",
		Version:  "1.0.0",
		Apps:     []string{"testdata/sample-app"},
		Platform: "linux-x64",
	})

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  predeployResult.DatabasePath,
		StoragePath:   predeployResult.StoragePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify storage directory exists in bundle
	bundleStoragePath := filepath.Join(outputDir, "storage")
	info, err := os.Stat(bundleStoragePath)
	require.NoError(t, err, "storage directory should exist in bundle")
	assert.True(t, info.IsDir(), "storage should be a directory")

	// Log what's in the bundle storage directory
	bundleEntries, _ := os.ReadDir(bundleStoragePath)
	t.Logf("Bundle storage contains %d entries", len(bundleEntries))
	for _, entry := range bundleEntries {
		t.Logf("  - %s (dir=%v)", entry.Name(), entry.IsDir())
	}

	// Count files recursively in storage
	fileCount := 0
	err = filepath.Walk(bundleStoragePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})
	require.NoError(t, err)
	t.Logf("Total files in bundle storage: %d", fileCount)
}

// TestIntegration_BundleContents_StorageWithMockedFiles verifies storage directory
// copying works correctly with actual files
func TestIntegration_BundleContents_StorageWithMockedFiles(t *testing.T) {
	// This test doesn't require Docker - it tests the bundle.Create storage copying
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Create mock predeploy outputs
	fakeBackend := filepath.Join(tmpDir, "fake-backend")
	err := os.WriteFile(fakeBackend, []byte("fake"), 0755)
	require.NoError(t, err)

	fakeDB := filepath.Join(tmpDir, "convex.db")
	err = os.WriteFile(fakeDB, []byte("fake sqlite db"), 0644)
	require.NoError(t, err)

	// Create a storage directory with nested files
	storagePath := filepath.Join(tmpDir, "storage")
	err = os.MkdirAll(filepath.Join(storagePath, "modules"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(storagePath, "files", "uploads"), 0755)
	require.NoError(t, err)

	// Create some mock files
	err = os.WriteFile(filepath.Join(storagePath, "modules", "module1.js"), []byte("export function test() {}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(storagePath, "modules", "module2.js"), []byte("export const data = {}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(storagePath, "files", "config.json"), []byte("{}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(storagePath, "files", "uploads", "image.png"), []byte("fake png data"), 0644)
	require.NoError(t, err)

	creds, err := credentials.Generate("test")
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Storage Copy Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	err = bundle.Create(bundle.Options{
		OutputDir:     outputDir,
		BackendBinary: fakeBackend,
		DatabasePath:  fakeDB,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify all storage files were copied
	bundleStorage := filepath.Join(outputDir, "storage")

	assert.FileExists(t, filepath.Join(bundleStorage, "modules", "module1.js"))
	assert.FileExists(t, filepath.Join(bundleStorage, "modules", "module2.js"))
	assert.FileExists(t, filepath.Join(bundleStorage, "files", "config.json"))
	assert.FileExists(t, filepath.Join(bundleStorage, "files", "uploads", "image.png"))

	// Verify content was preserved
	content, err := os.ReadFile(filepath.Join(bundleStorage, "modules", "module1.js"))
	require.NoError(t, err)
	assert.Equal(t, "export function test() {}", string(content))

	content, err = os.ReadFile(filepath.Join(bundleStorage, "files", "uploads", "image.png"))
	require.NoError(t, err)
	assert.Equal(t, "fake png data", string(content))
}


