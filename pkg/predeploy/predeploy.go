package predeploy

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	adminkey "github.com/ozanturksever/convex-admin-key"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Options for running pre-deployment
type Options struct {
	Apps          []string
	BackendBinary string
	OutputDir     string
	Platform      string // Target platform for the backend binary (e.g., "linux-x64", "linux-arm64")
	DockerImage   string // Custom Docker image to use (default: convex-predeploy:latest)
}

// Default Docker image for pre-deployment
// This image has all dependencies pre-installed (curl, unzip, convex CLI, convex-local-backend)
const DefaultPredeployImage = "convex-predeploy:latest"

// Backend release information (used when building the Docker image)
const (
	backendReleaseTag  = "precompiled-2025-12-12-73e805a"
	backendDownloadURL = "https://github.com/get-convex/convex-backend/releases/download/%s/convex-local-backend-%s.zip"
)

// Container paths for database and storage
const (
	containerDataDir     = "/convex-data"
	containerDBPath      = "/convex-data/convex.db"
	containerStoragePath = "/convex-data/storage"
)

// getPlatformString converts our platform names to the release artifact platform strings
// This is used when the custom image is not available and we need to download the binary
func getPlatformString(platform string, containerArch string) string {
	// If container architecture is detected, use it
	if containerArch != "" {
		switch containerArch {
		case "aarch64", "arm64":
			return "aarch64-unknown-linux-gnu"
		case "x86_64", "amd64":
			return "x86_64-unknown-linux-gnu"
		}
	}

	// Fall back to platform flag
	switch platform {
	case "linux-arm64":
		return "aarch64-unknown-linux-gnu"
	case "linux-x64", "":
		return "x86_64-unknown-linux-gnu"
	default:
		return "x86_64-unknown-linux-gnu"
	}
}

// isPredeployImage checks if the image is our custom pre-deploy image with dependencies pre-installed
func isPredeployImage(image string) bool {
	return strings.Contains(image, "convex-predeploy")
}

// Result from pre-deployment
type Result struct {
	DatabasePath string
	StoragePath  string
}

// Run executes the pre-deployment process using Docker
func Run(opts Options) (*Result, error) {
	ctx := context.Background()

	// Create output directories
	databasePath := filepath.Join(opts.OutputDir, "convex.db")
	storagePath := filepath.Join(opts.OutputDir, "storage")

	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Get absolute paths for apps
	var absApps []string
	for _, app := range opts.Apps {
		absApp, err := filepath.Abs(app)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for app %s: %w", app, err)
		}
		absApps = append(absApps, absApp)
	}

	// Check if a backend binary was provided and exists
	var useProvidedBinary bool
	var absBackendBinary string
	if opts.BackendBinary != "" {
		var absErr error
		absBackendBinary, absErr = filepath.Abs(opts.BackendBinary)
		if absErr != nil {
			return nil, fmt.Errorf("failed to get absolute path for backend binary: %w", absErr)
		}
		if _, statErr := os.Stat(absBackendBinary); statErr == nil {
			useProvidedBinary = true
		}
	}

	// Create bind mounts for apps
	var mounts testcontainers.ContainerMounts
	for i, app := range absApps {
		mounts = append(mounts,
			testcontainers.BindMount(app, testcontainers.ContainerMountTarget(fmt.Sprintf("/app%d", i))),
		)
	}

	// If backend binary is provided, mount it into the container
	if useProvidedBinary {
		mounts = append(mounts,
			testcontainers.BindMount(absBackendBinary, testcontainers.ContainerMountTarget("/usr/local/bin/convex-local-backend")),
		)
	}

	// Determine which Docker image to use
	dockerImage := opts.DockerImage
	if dockerImage == "" {
		dockerImage = DefaultPredeployImage
	}
	usePredeployImage := isPredeployImage(dockerImage)

	// Create container request
	req := testcontainers.ContainerRequest{
		Image:        dockerImage,
		ExposedPorts: []string{"3210/tcp"},
		Cmd:          []string{"sh", "-c", "sleep infinity"},
		WaitingFor:   wait.ForExec([]string{"true"}).WithStartupTimeout(60 * time.Second),
		Mounts:       mounts,
	}

	// Start container
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	defer container.Terminate(ctx)

	var exitCode int
	var output io.Reader

	// If not using pre-deploy image, install dependencies manually
	if !usePredeployImage {
		// Install required tools (curl, unzip) - only needed if we need to download
		if !useProvidedBinary {
			exitCode, output, err = container.Exec(ctx, []string{
				"sh", "-c", "apt-get update && apt-get install -y curl unzip",
			})
			if err != nil || exitCode != 0 {
				return nil, fmt.Errorf("failed to install required tools: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
			}
		}

		// Install convex CLI
		exitCode, output, err = container.Exec(ctx, []string{
			"sh", "-c", "npm install -g convex",
		})
		if err != nil || exitCode != 0 {
			return nil, fmt.Errorf("failed to install convex CLI: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
		}

		// Download the backend binary only if not provided via mount
		if !useProvidedBinary {
			// Detect container architecture using shell command to capture output properly
			exitCode, archOutput, err := container.Exec(ctx, []string{"sh", "-c", "uname -m"})
			var containerArch string
			if err == nil && exitCode == 0 {
				archStr := readOutput(archOutput)
				// Clean up the output - remove control characters and whitespace
				containerArch = strings.TrimSpace(archStr)
				// Handle common arch strings
				if strings.Contains(containerArch, "aarch64") {
					containerArch = "aarch64"
				} else if strings.Contains(containerArch, "x86_64") {
					containerArch = "x86_64"
				}
			}

			// Download the Linux backend binary inside the container
			platformStr := getPlatformString(opts.Platform, containerArch)
			downloadURL := fmt.Sprintf(backendDownloadURL, backendReleaseTag, platformStr)
			downloadCmd := fmt.Sprintf(
				"curl -L -o /tmp/convex-local-backend.zip '%s' && "+
					"unzip -o /tmp/convex-local-backend.zip -d /usr/local/bin && "+
					"chmod +x /usr/local/bin/convex-local-backend && "+
					"rm /tmp/convex-local-backend.zip",
				downloadURL,
			)
			exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", downloadCmd})
			if err != nil || exitCode != 0 {
				return nil, fmt.Errorf("failed to download backend binary: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
			}
		}
	}

	// If using provided binary, make sure it's executable in the container
	if useProvidedBinary {
		exitCode, output, err = container.Exec(ctx, []string{
			"sh", "-c", "chmod +x /usr/local/bin/convex-local-backend",
		})
		if err != nil || exitCode != 0 {
			return nil, fmt.Errorf("failed to make backend binary executable: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
		}
	}

	// Create data directory in container
	exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("mkdir -p %s %s", containerDataDir, containerStoragePath)})
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to create data directory: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
	}

	// Start the backend and wait for it to be ready in a single exec call
	// Using sh -c with & and a polling loop ensures the process stays running
	// Note: instance-secret must be a valid 64-character hex string (32 bytes)
	// The admin key format for local backend is: instanceName|deployKeySecret
	const instanceSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	startAndWaitCmd := fmt.Sprintf(`/usr/local/bin/convex-local-backend %s --port 3210 --instance-name test --instance-secret %s --local-storage %s > /tmp/backend.log 2>&1 &
for i in $(seq 1 30); do
  # Check if curl can reach the backend (any response means it's ready)
  if curl -sf http://localhost:3210/version > /dev/null 2>&1; then
    echo "Backend is ready"
    exit 0
  fi
  sleep 1
done
echo "Backend failed to start"
cat /tmp/backend.log 2>/dev/null || true
exit 1`, containerDBPath, instanceSecret, containerStoragePath)
	exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", startAndWaitCmd})
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to start backend: %v (exit code: %d, output: %s)", err, exitCode, readOutput(output))
	}

	// Deploy each app using the convex-admin-key library to generate a proper admin key
	for i := range absApps {
		appDir := fmt.Sprintf("/app%d", i)
		// Generate admin key using the convex-admin-key library
		secret, err := adminkey.ParseSecret(instanceSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to parse instance secret: %w", err)
		}
		adminKey, err := adminkey.IssueAdminKey(secret, "test", 0, false)
		if err != nil {
			return nil, fmt.Errorf("failed to generate admin key: %w", err)
		}

		// Install app dependencies first, then deploy
		deployCmd := fmt.Sprintf(
			"cd %s && npm install --silent && npx convex deploy --admin-key '%s' --url http://localhost:3210 --yes",
			appDir,
			adminKey,
		)
		exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", deployCmd})
		if err != nil || exitCode != 0 {
			return nil, fmt.Errorf("failed to deploy app %d: %v (exit code: %d, output: %s)", i, err, exitCode, readOutput(output))
		}
	}

	// Verify the database file exists in the container before trying to copy it
	exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("ls -la %s", containerDBPath)})
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("database file not found at %s: %v (exit code: %d, output: %s)", containerDBPath, err, exitCode, readOutput(output))
	}

	// Copy database out of container using CopyFileFromContainer
	// This returns a tar stream that we need to extract
	reader, err := container.CopyFileFromContainer(ctx, containerDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy database from container at %s: %w", containerDBPath, err)
	}
	defer reader.Close()

	// Read the entire stream into a buffer first to avoid issues with partial reads
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read tar stream: %w", err)
	}

	// Extract file from tar archive
	if err := extractTarFile(bytes.NewReader(buf), databasePath); err != nil {
		return nil, fmt.Errorf("failed to extract database: %w", err)
	}

	return &Result{
		DatabasePath: databasePath,
		StoragePath:  storagePath,
	}, nil
}

func readOutput(reader io.Reader) string {
	if reader == nil {
		return ""
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Sprintf("(error reading output: %v)", err)
	}
	return string(data)
}


// extractTarFile extracts a single file from a tar stream and writes it to destPath
func extractTarFile(reader io.Reader, destPath string) error {
	// Read first few bytes to check if it's actually a tar file
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Check for tar magic bytes (at offset 257: "ustar")
	// If not a valid tar, the data might be the raw file content
	isTar := false
	if len(data) > 262 {
		magic := string(data[257:262])
		if magic == "ustar" {
			isTar = true
		}
	}

	if !isTar {
		// Not a tar file, write the data directly
		// This can happen if the container returns raw file data
		return os.WriteFile(destPath, data, 0644)
	}

	// It's a tar file, extract it
	tr := tar.NewReader(bytes.NewReader(data))

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Accept both regular files and type flag 0 (which some tar implementations use)
		if header.Typeflag == tar.TypeReg || header.Typeflag == 0 {
			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}

			_, copyErr := io.Copy(outFile, tr)
			outFile.Close()
			if copyErr != nil {
				return fmt.Errorf("failed to write file content: %w", copyErr)
			}
			return nil
		}
	}

	return fmt.Errorf("no regular file found in tar archive")
}
