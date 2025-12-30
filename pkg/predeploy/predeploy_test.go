package predeploy

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // SQLite driver for database validation
)

func TestRun_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container test in short mode")
	}

	// This test requires:
	// 1. Docker to be running
	// 2. A valid Convex app in testdata/sample-app
	// Note: Backend binary is downloaded inside the container, so no local binary needed

	tmpDir := t.TempDir()

	result, err := Run(Options{
		Apps:          []string{"../../testdata/sample-app"},
		BackendBinary: "", // Not needed for pre-deployment, downloaded in container
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim", // Use base image for testing (will download deps)
	})
	require.NoError(t, err)

	assert.NotEmpty(t, result.DatabasePath)
	assert.FileExists(t, result.DatabasePath)
	assert.DirExists(t, result.StoragePath)
}

func TestRun_Integration_ValidSQLiteDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container test in short mode")
	}

	// This test verifies that the database file created by predeploy
	// contains valid SQLite data that can be opened and queried.

	tmpDir := t.TempDir()

	result, err := Run(Options{
		Apps:          []string{"../../testdata/sample-app"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim",
	})
	require.NoError(t, err)

	// Verify the database file exists
	require.FileExists(t, result.DatabasePath)

	// Open the database to verify it's valid SQLite
	db, err := sql.Open("sqlite", result.DatabasePath)
	require.NoError(t, err, "Database should be openable as SQLite")
	defer db.Close()

	// Ping to verify the connection works
	err = db.Ping()
	require.NoError(t, err, "Database should respond to ping")

	// Query sqlite_master to verify the database has tables
	// This confirms it's not just an empty SQLite file
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	require.NoError(t, err, "Should be able to query sqlite_master")
	defer rows.Close()

	// Collect table names
	var tables []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err)
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())

	// The database should have at least one table after deployment
	assert.NotEmpty(t, tables, "Database should contain tables after predeploy")
	t.Logf("Database contains %d tables: %v", len(tables), tables)
}

func TestOptions(t *testing.T) {
	opts := Options{
		Apps:          []string{"/app1", "/app2"},
		BackendBinary: "/path/to/backend",
		OutputDir:     "/output",
	}

	assert.Equal(t, 2, len(opts.Apps))
	assert.Equal(t, "/path/to/backend", opts.BackendBinary)
	assert.Equal(t, "/output", opts.OutputDir)
}

func TestResult(t *testing.T) {
	result := Result{
		DatabasePath: "/output/convex.db",
		StoragePath:  "/output/storage",
	}

	assert.Equal(t, "/output/convex.db", result.DatabasePath)
	assert.Equal(t, "/output/storage", result.StoragePath)
}

func TestGetPlatformString(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		containerArch string
		expected      string
	}{
		{"linux-x64 with empty arch", "linux-x64", "", "x86_64-unknown-linux-gnu"},
		{"linux-arm64 with empty arch", "linux-arm64", "", "aarch64-unknown-linux-gnu"},
		{"empty platform with empty arch", "", "", "x86_64-unknown-linux-gnu"},
		{"unknown platform with empty arch", "unknown", "", "x86_64-unknown-linux-gnu"},
		{"detect aarch64 container", "linux-x64", "aarch64", "aarch64-unknown-linux-gnu"},
		{"detect x86_64 container", "linux-arm64", "x86_64", "x86_64-unknown-linux-gnu"},
		{"detect arm64 container", "", "arm64", "aarch64-unknown-linux-gnu"},
		{"detect amd64 container", "", "amd64", "x86_64-unknown-linux-gnu"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPlatformString(tt.platform, tt.containerArch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPredeployImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected bool
	}{
		{"default predeploy image", "convex-predeploy:latest", true},
		{"registry predeploy image", "ghcr.io/ozanturksever/convex-predeploy:v1.0.0", true},
		{"node slim image", "node:20-slim", false},
		{"empty string", "", false},
		{"other image", "ubuntu:22.04", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPredeployImage(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultPredeployImage(t *testing.T) {
	assert.Equal(t, "convex-predeploy:latest", DefaultPredeployImage)
}

func TestUseProvidedBinary(t *testing.T) {
	// Create a temporary "binary" file
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "convex-local-backend")
	err := os.WriteFile(binaryPath, []byte("fake binary"), 0755)
	require.NoError(t, err)

	// Verify the file exists
	_, err = os.Stat(binaryPath)
	assert.NoError(t, err)

	// Test that Options can hold the BackendBinary path
	opts := Options{
		Apps:          []string{"/app"},
		BackendBinary: binaryPath,
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim",
	}

	assert.Equal(t, binaryPath, opts.BackendBinary)
}

func TestRun_StorageDirectoryCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container test in short mode")
	}

	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")

	// Verify storage path doesn't exist yet
	_, err := os.Stat(storagePath)
	assert.True(t, os.IsNotExist(err))

	// The Run function should create the storage directory
	// (we can't fully test without Docker, but we can verify the Options are valid)
	opts := Options{
		Apps:          []string{"/nonexistent"},
		BackendBinary: "",
		OutputDir:     tmpDir,
		Platform:      "linux-x64",
		DockerImage:   "node:20-slim", // Use base image for testing
	}

	// This will fail because the app doesn't exist, but the directory should be created
	_, _ = Run(opts)

	// Storage directory should have been created even if the rest failed
	_, err = os.Stat(storagePath)
	assert.NoError(t, err)
}
