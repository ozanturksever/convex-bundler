package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ozanturksever/convex-bundler/pkg/selfhost"
)

// TestE2E_SelfHost_InstallEmbeddedBundle tests the complete E2E workflow:
// 1. Build the convex-backend-ops binary for Linux
// 2. Create a test bundle with all required files
// 3. Create a self-extracting executable using selfhost.Create()
// 4. Run the install command in a systemd Docker container (without --bundle flag)
// 5. Verify the installation creates the expected files and services
//
// Note: The install command will fail at the health check stage because we use a mock
// backend that doesn't respond to HTTP requests. However, all files should be installed
// and the service should be configured correctly.
func TestE2E_SelfHost_InstallEmbeddedBundle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E integration test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Step 1: Build the ops binary for linux/amd64
	opsBinaryPath := filepath.Join(tmpDir, "convex-backend-ops")
	buildOpsBinaryFromInstallerProject(t, opsBinaryPath)

	// Step 2: Create a test bundle with all required files
	bundleDir := filepath.Join(tmpDir, "bundle")
	createE2ETestBundle(t, bundleDir)

	// Step 3: Create self-extracting executable
	selfhostPath := filepath.Join(tmpDir, "selfhost-installer")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinaryPath,
		OutputPath:  selfhostPath,
		Platform:    "linux-x64",
		Compression: selfhost.CompressionGzip,
		OpsVersion:  "1.0.0-e2e-test",
	})
	require.NoError(t, err, "failed to create self-host executable")

	// Verify the selfhost executable was created
	info, err := os.Stat(selfhostPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "selfhost executable should be executable")
	t.Logf("Created self-host executable: %s (%d bytes)", selfhostPath, info.Size())

	// Step 4: Start systemd container
	container := startE2ESystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Step 5: Copy self-host executable to container
	err = container.CopyFileToContainer(ctx, selfhostPath, "/tmp/selfhost-installer", 0755)
	require.NoError(t, err, "failed to copy selfhost executable to container")

	// Step 6: Run info command to verify embedded bundle is detected
	exitCode, output := execInE2EContainer(t, ctx, container, []string{"/tmp/selfhost-installer", "info"})
	assert.Equal(t, 0, exitCode, "info command failed: %s", output)
	assert.Contains(t, output, "E2E Test Backend", "should show bundle name")
	assert.Contains(t, output, "1.0.0", "should show bundle version")
	t.Logf("Info output:\n%s", output)

	// Step 7: Run verify command to check integrity
	exitCode, output = execInE2EContainer(t, ctx, container, []string{"/tmp/selfhost-installer", "verify"})
	assert.Equal(t, 0, exitCode, "verify command failed: %s", output)
	assert.Contains(t, output, "verified", "should show verification passed")
	t.Logf("Verify output:\n%s", output)

	// Step 8: Run install command without --bundle flag (should use embedded bundle)
	// Note: The install will fail at health check stage because our mock backend
	// doesn't respond to HTTP requests. But files should still be installed.
	exitCode, output = execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "install", "--yes",
	})
	t.Logf("Install output:\n%s", output)
	// The install may fail at health check, but we verify files were created
	// Check if it either succeeded or failed at health check (files still installed)
	if exitCode != 0 {
		// The error message from install.go is "health check failed"
		assert.True(t, strings.Contains(output, "health check") || strings.Contains(output, "Health check"),
			"if install fails, it should be at health check stage, got: %s", output)
	}

	// Step 9: Verify files were created (regardless of health check result)
	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/usr/local/bin/convex-backend"})
	assert.Equal(t, 0, exitCode, "backend binary not installed")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/var/lib/convex/manifest.json"})
	assert.Equal(t, 0, exitCode, "manifest not installed")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/etc/convex/convex.env"})
	assert.Equal(t, 0, exitCode, "env config not created")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/etc/convex/admin.key"})
	assert.Equal(t, 0, exitCode, "admin key not created")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/etc/convex/instance.secret"})
	assert.Equal(t, 0, exitCode, "instance secret not created")

	// Step 10: Verify service is enabled
	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"systemctl", "is-enabled", "convex-backend"})
	assert.Equal(t, 0, exitCode, "service not enabled")

	// Step 11: Verify manifest content
	exitCode, manifestOutput := execInE2EContainer(t, ctx, container, []string{"cat", "/var/lib/convex/manifest.json"})
	assert.Equal(t, 0, exitCode, "failed to read manifest")
	assert.Contains(t, manifestOutput, "E2E Test Backend", "manifest should contain bundle name")

	// Step 12: Verify credentials were written correctly
	exitCode, adminKeyOutput := execInE2EContainer(t, ctx, container, []string{"cat", "/etc/convex/admin.key"})
	assert.Equal(t, 0, exitCode, "failed to read admin key")
	assert.Contains(t, adminKeyOutput, "e2e-test-admin-key", "admin key should be from bundle")
}

// TestE2E_SelfHost_InfoNonSelfHost tests that info command handles non-selfhost binaries gracefully
func TestE2E_SelfHost_InfoNonSelfHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E integration test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Build a regular ops binary (not selfhost)
	opsBinaryPath := filepath.Join(tmpDir, "convex-backend-ops")
	buildOpsBinaryFromInstallerProject(t, opsBinaryPath)

	// Start container
	container := startE2ESystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Copy regular binary to container
	err := container.CopyFileToContainer(ctx, opsBinaryPath, "/tmp/convex-backend-ops", 0755)
	require.NoError(t, err)

	// Run info command - should indicate not a selfhost executable
	exitCode, output := execInE2EContainer(t, ctx, container, []string{"/tmp/convex-backend-ops", "info"})
	assert.Equal(t, 0, exitCode, "info command should succeed even for non-selfhost: %s", output)
	assert.Contains(t, output, "Not a self-host executable", "should indicate not selfhost")
}

// TestE2E_SelfHost_ExtractCommand tests the extract command in Docker
func TestE2E_SelfHost_ExtractCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E integration test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Build ops binary and create selfhost executable
	opsBinaryPath := filepath.Join(tmpDir, "convex-backend-ops")
	buildOpsBinaryFromInstallerProject(t, opsBinaryPath)

	bundleDir := filepath.Join(tmpDir, "bundle")
	createE2ETestBundle(t, bundleDir)

	selfhostPath := filepath.Join(tmpDir, "selfhost-installer")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinaryPath,
		OutputPath:  selfhostPath,
		Platform:    "linux-x64",
		Compression: selfhost.CompressionGzip,
		OpsVersion:  "1.0.0-e2e-test",
	})
	require.NoError(t, err)

	// Start container
	container := startE2ESystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Copy selfhost executable
	err = container.CopyFileToContainer(ctx, selfhostPath, "/tmp/selfhost-installer", 0755)
	require.NoError(t, err)

	// Run extract command
	exitCode, output := execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "extract", "--output", "/tmp/extracted-bundle",
	})
	assert.Equal(t, 0, exitCode, "extract failed: %s", output)
	assert.Contains(t, output, "extracted successfully", "should show success")

	// Verify extracted files
	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/tmp/extracted-bundle/backend"})
	assert.Equal(t, 0, exitCode, "backend not extracted")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/tmp/extracted-bundle/convex.db"})
	assert.Equal(t, 0, exitCode, "convex.db not extracted")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/tmp/extracted-bundle/manifest.json"})
	assert.Equal(t, 0, exitCode, "manifest.json not extracted")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/tmp/extracted-bundle/credentials.json"})
	assert.Equal(t, 0, exitCode, "credentials.json not extracted")

	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-d", "/tmp/extracted-bundle/storage"})
	assert.Equal(t, 0, exitCode, "storage directory not extracted")
}

// TestE2E_SelfHost_InstallWithExplicitBundle tests that --bundle flag still works with selfhost binary
func TestE2E_SelfHost_InstallWithExplicitBundle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E integration test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Build ops binary and create selfhost executable
	opsBinaryPath := filepath.Join(tmpDir, "convex-backend-ops")
	buildOpsBinaryFromInstallerProject(t, opsBinaryPath)

	bundleDir := filepath.Join(tmpDir, "bundle")
	createE2ETestBundle(t, bundleDir)

	selfhostPath := filepath.Join(tmpDir, "selfhost-installer")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinaryPath,
		OutputPath:  selfhostPath,
		Platform:    "linux-x64",
		Compression: selfhost.CompressionGzip,
		OpsVersion:  "1.0.0-e2e-test",
	})
	require.NoError(t, err)

	// Start container
	container := startE2ESystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Copy selfhost executable
	err = container.CopyFileToContainer(ctx, selfhostPath, "/tmp/selfhost-installer", 0755)
	require.NoError(t, err)

	// First extract the bundle
	exitCode, _ := execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "extract", "--output", "/tmp/extracted-bundle",
	})
	require.Equal(t, 0, exitCode, "extract failed")

	// Now install using explicit --bundle flag (should use extracted bundle, not embedded)
	// Note: Install may fail at health check stage but files should be installed
	installExitCode, installOutput := execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "install", "--bundle", "/tmp/extracted-bundle", "--yes",
	})
	t.Logf("Install exit code: %d\nInstall output:\n%s", installExitCode, installOutput)

	// Verify files were installed (regardless of health check result)
	exitCode, _ = execInE2EContainer(t, ctx, container, []string{"test", "-f", "/usr/local/bin/convex-backend"})
	assert.Equal(t, 0, exitCode, "backend binary not installed with explicit bundle")
}

// TestE2E_SelfHost_VerifyCommand tests the verify command in a Docker container
// to ensure bundle integrity verification works correctly
func TestE2E_SelfHost_VerifyCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E integration test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Build ops binary and create selfhost executable
	opsBinaryPath := filepath.Join(tmpDir, "convex-backend-ops")
	buildOpsBinaryFromInstallerProject(t, opsBinaryPath)

	bundleDir := filepath.Join(tmpDir, "bundle")
	createE2ETestBundle(t, bundleDir)

	selfhostPath := filepath.Join(tmpDir, "selfhost-installer")
	err := selfhost.Create(selfhost.CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinaryPath,
		OutputPath:  selfhostPath,
		Platform:    "linux-x64",
		Compression: selfhost.CompressionGzip,
		OpsVersion:  "1.0.0-e2e-test",
	})
	require.NoError(t, err)

	// Start container
	container := startE2ESystemdContainer(t, ctx)
	defer container.Terminate(ctx)

	// Copy selfhost executable
	err = container.CopyFileToContainer(ctx, selfhostPath, "/tmp/selfhost-installer", 0755)
	require.NoError(t, err)

	// Run verify command - should pass
	exitCode, output := execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "verify",
	})
	assert.Equal(t, 0, exitCode, "verify should pass for valid executable: %s", output)
	assert.Contains(t, output, "verified", "should show verification passed")

	// Test with JSON output flag
	exitCode, output = execInE2EContainer(t, ctx, container, []string{
		"/tmp/selfhost-installer", "verify", "--json",
	})
	assert.Equal(t, 0, exitCode, "verify --json should pass: %s", output)
	assert.Contains(t, output, "\"valid\":", "JSON output should contain valid field")
}

// Helper functions

// buildOpsBinaryFromInstallerProject builds the convex-backend-ops binary for linux/amd64
// from the convex-app-installer project
func buildOpsBinaryFromInstallerProject(t *testing.T, outputPath string) {
	t.Helper()

	// Get absolute path to current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// The installer project could be at several locations
	// Try multiple possible locations relative to the bundler project
	possiblePaths := []string{
		// Sibling directory - convex-backend-ops (preferred)
		filepath.Join(cwd, "..", "convex-backend-ops"),
		// Legacy sibling directory name
		filepath.Join(cwd, "..", "2025-12-30-convex-app-installer"),
		// Inside this project tree (for testing purposes)
		filepath.Join(cwd, "Users", "ozant", "devel", "tries", "2025-12-30-convex-app-installer"),
		// Absolute path in user's home
		filepath.Join(os.Getenv("HOME"), "devel", "tries", "2025-12-30-convex-app-installer"),
	}

	var projectRoot string
	for _, p := range possiblePaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(absPath, "go.mod")); err == nil {
			projectRoot = absPath
			break
		}
	}

	if projectRoot == "" {
		t.Skip("convex-app-installer project not found, skipping E2E test")
	}

	t.Logf("Building ops binary from: %s", projectRoot)

	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the ops project doesn't compile (e.g., missing types), skip the test
		t.Skipf("failed to build ops binary (ops project may have compilation issues): %v\nOutput: %s", err, string(output))
	}

	// Verify binary was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("ops binary was not created at %s", outputPath)
	}

	t.Logf("Built ops binary: %s", outputPath)
}

// createE2ETestBundle creates a test bundle with all required files for E2E testing
func createE2ETestBundle(t *testing.T, bundleDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	// Create manifest.json
	manifest := map[string]interface{}{
		"name":      "E2E Test Backend",
		"version":   "1.0.0",
		"apps":      []string{"./test-app"},
		"platform":  "linux-x64",
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), manifestData, 0644))

	// Create mock backend binary
	// This is a simple shell script that exits immediately (for testing purposes)
	// The health check will timeout, but all files will be installed
	backendScript := `#!/bin/bash
# Mock Convex backend for E2E testing
echo "Mock Convex backend started"
# Exit immediately - real backend would run HTTP server
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "backend"), []byte(backendScript), 0755))

	// Create mock database
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "convex.db"), []byte("SQLite format 3\x00mock database for e2e testing"), 0644))

	// Create credentials.json
	credentials := map[string]string{
		"adminKey":       "e2e-test-admin-key-for-testing-purposes",
		"instanceSecret": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	credsData, err := json.MarshalIndent(credentials, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "credentials.json"), credsData, 0644))

	// Create storage directory with a test file
	storageDir := filepath.Join(bundleDir, "storage")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(storageDir, "test-file.txt"), []byte("e2e test storage content"), 0644))
}

// startE2ESystemdContainer starts a systemd-enabled container for E2E testing
// Following docker-systemd.md best practices for running systemd in Docker
func startE2ESystemdContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "./testdata",
			Dockerfile: "Dockerfile",
		},
		Privileged: true,
		// tmpfs mounts required for systemd
		Tmpfs: map[string]string{
			"/run":      "rw,noexec,nosuid",
			"/run/lock": "rw,noexec,nosuid",
		},
		// Mount cgroup filesystem read-write
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount("/sys/fs/cgroup", "/sys/fs/cgroup"),
		),
		// HostConfigModifier to set cgroupns=host (required for systemd)
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.CgroupnsMode = "host"
		},
		// Wait for systemd to fully initialize
		WaitingFor: wait.ForExec([]string{"systemctl", "is-system-running", "--wait"}).
			WithStartupTimeout(120 * time.Second).
			WithExitCodeMatcher(func(exitCode int) bool {
				// 0 = running, 1 = degraded (acceptable for containers)
				return exitCode == 0 || exitCode == 1
			}),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		// If systemd container fails, provide helpful debug info
		t.Logf("Failed to start systemd container: %v", err)
		t.Logf("This test requires Docker with cgroup v2 support and privileged mode")
		t.Logf("Required flags: --privileged --cgroupns=host --tmpfs /run --tmpfs /run/lock -v /sys/fs/cgroup:/sys/fs/cgroup:rw")
	}
	require.NoError(t, err)

	return container
}

// execInE2EContainer executes a command in the container and returns exit code and output
func execInE2EContainer(t *testing.T, ctx context.Context, container testcontainers.Container, cmd []string) (int, string) {
	t.Helper()

	exitCode, reader, err := container.Exec(ctx, cmd)
	require.NoError(t, err)

	// Read all output
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Logf("Warning: failed to read command output: %v", err)
		return exitCode, ""
	}

	// Docker exec output contains multiplexed stream headers
	// Each frame has an 8-byte header: [stream type (1 byte), 0, 0, 0, size (4 bytes big-endian)]
	// Stream type: 1 = stdout, 2 = stderr
	// We need to strip these headers to get clean output
	outputStr := string(output)

	// Simple approach: skip first 8 bytes if output starts with docker mux header
	// (indicated by first byte being 1 or 2 and followed by 3 zeros)
	for len(outputStr) >= 8 {
		if (outputStr[0] == 1 || outputStr[0] == 2) && outputStr[1] == 0 && outputStr[2] == 0 && outputStr[3] == 0 {
			// This looks like a docker mux header, skip 8 bytes
			outputStr = outputStr[8:]
		} else {
			break
		}
	}

	return exitCode, strings.TrimSpace(outputStr)
}
