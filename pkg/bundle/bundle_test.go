package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ozanturksever/convex-bundler/pkg/credentials"
	"github.com/ozanturksever/convex-bundler/pkg/manifest"
)

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	// Create fake source files
	backendBinary := filepath.Join(tmpDir, "fake-backend")
	err := os.WriteFile(backendBinary, []byte("fake backend binary"), 0755)
	require.NoError(t, err)

	databasePath := filepath.Join(tmpDir, "convex.db")
	err = os.WriteFile(databasePath, []byte("fake database"), 0644)
	require.NoError(t, err)

	storagePath := filepath.Join(tmpDir, "storage")
	err = os.MkdirAll(storagePath, 0755)
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Test Bundle",
		Version:  "1.0.0",
		Apps:     []string{"/app1"},
		Platform: "linux-x64",
	})

	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	err = Create(Options{
		OutputDir:     outputDir,
		BackendBinary: backendBinary,
		DatabasePath:  databasePath,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify bundle structure
	assertBundleContents(t, outputDir, mf, creds)
}

func TestCreate_BackendExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	backendBinary := filepath.Join(tmpDir, "backend")
	err := os.WriteFile(backendBinary, []byte("binary"), 0644) // Not executable initially
	require.NoError(t, err)

	databasePath := filepath.Join(tmpDir, "db")
	err = os.WriteFile(databasePath, []byte("db"), 0644)
	require.NoError(t, err)

	storagePath := filepath.Join(tmpDir, "storage")
	err = os.MkdirAll(storagePath, 0755)
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	err = Create(Options{
		OutputDir:     outputDir,
		BackendBinary: backendBinary,
		DatabasePath:  databasePath,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify backend is executable
	backendDest := filepath.Join(outputDir, "backend")
	info, err := os.Stat(backendDest)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "backend should be executable")
}

func TestCreate_StorageWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	backendBinary := filepath.Join(tmpDir, "backend")
	err := os.WriteFile(backendBinary, []byte("binary"), 0755)
	require.NoError(t, err)

	databasePath := filepath.Join(tmpDir, "db")
	err = os.WriteFile(databasePath, []byte("db"), 0644)
	require.NoError(t, err)

	// Create storage with some files
	storagePath := filepath.Join(tmpDir, "storage")
	err = os.MkdirAll(storagePath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(storagePath, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(storagePath, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(storagePath, "subdir", "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	err = Create(Options{
		OutputDir:     outputDir,
		BackendBinary: backendBinary,
		DatabasePath:  databasePath,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.NoError(t, err)

	// Verify storage files were copied
	storageDest := filepath.Join(outputDir, "storage")
	assert.FileExists(t, filepath.Join(storageDest, "file1.txt"))
	assert.FileExists(t, filepath.Join(storageDest, "subdir", "file2.txt"))

	// Verify content
	content, err := os.ReadFile(filepath.Join(storageDest, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content1", string(content))
}

func TestCreate_MissingBackendBinary(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "bundle")

	databasePath := filepath.Join(tmpDir, "db")
	err := os.WriteFile(databasePath, []byte("db"), 0644)
	require.NoError(t, err)

	storagePath := filepath.Join(tmpDir, "storage")
	err = os.MkdirAll(storagePath, 0755)
	require.NoError(t, err)

	mf := manifest.New(manifest.Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"/app"},
		Platform: "linux-x64",
	})

	creds, err := credentials.Generate("test-instance")
	require.NoError(t, err)

	err = Create(Options{
		OutputDir:     outputDir,
		BackendBinary: "/nonexistent/backend",
		DatabasePath:  databasePath,
		StoragePath:   storagePath,
		Manifest:      mf,
		Credentials:   creds,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy backend binary")
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	err := os.WriteFile(src, []byte("hello world"), 0644)
	require.NoError(t, err)

	err = copyFile(src, dst)
	require.NoError(t, err)

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	err = copyDir(srcDir, dstDir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dstDir, "file1.txt"))
	assert.FileExists(t, filepath.Join(dstDir, "subdir", "file2.txt"))
}

// Helper function
func assertBundleContents(t *testing.T, outputDir string, expectedManifest *manifest.Manifest, expectedCreds *credentials.Credentials) {
	t.Helper()

	// Check backend binary exists and is executable
	backendPath := filepath.Join(outputDir, "backend")
	info, err := os.Stat(backendPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "backend should be executable")

	// Check convex.db exists
	_, err = os.Stat(filepath.Join(outputDir, "convex.db"))
	require.NoError(t, err)

	// Check storage directory exists
	info, err = os.Stat(filepath.Join(outputDir, "storage"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Check manifest.json
	manifestData, err := os.ReadFile(filepath.Join(outputDir, "manifest.json"))
	require.NoError(t, err)

	var mf manifest.Manifest
	err = json.Unmarshal(manifestData, &mf)
	require.NoError(t, err)
	assert.Equal(t, expectedManifest.Name, mf.Name)
	assert.Equal(t, expectedManifest.Version, mf.Version)

	// Check credentials.json
	credsData, err := os.ReadFile(filepath.Join(outputDir, "credentials.json"))
	require.NoError(t, err)

	var creds credentials.Credentials
	err = json.Unmarshal(credsData, &creds)
	require.NoError(t, err)
	assert.Equal(t, expectedCreds.AdminKey, creds.AdminKey)
	assert.Equal(t, expectedCreds.InstanceSecret, creds.InstanceSecret)
}
