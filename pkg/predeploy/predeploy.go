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

	// Create a temporary directory for pre-deployment output
	// We use a temp directory because bundle.Create will copy from here to the final location
	tempDir, err := os.MkdirTemp("", "convex-predeploy-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// Note: We don't clean up tempDir here because the caller needs the files
	// The caller should clean up after copying the files

	databasePath := filepath.Join(tempDir, "convex.db")
	storagePath := filepath.Join(tempDir, "storage")

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

	// Verify the database file exists in the container and get its size
	exitCode, output, err = container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("ls -la %s && stat -c %%s %s", containerDBPath, containerDBPath)})
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("database file not found at %s: %v (exit code: %d, output: %s)", containerDBPath, err, exitCode, readOutput(output))
	}

	// Use CopyFileFromContainer to get the database
	// This is more reliable than base64 encoding through exec
	reader, err := container.CopyFileFromContainer(ctx, containerDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy database from container: %w", err)
	}
	defer reader.Close()

	// Read the tar stream
	tarData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read tar data: %w", err)
	}

	if len(tarData) == 0 {
		return nil, fmt.Errorf("received empty tar data from container")
	}

	// Extract the database from the tar archive
	if err := extractTarFile(bytes.NewReader(tarData), databasePath); err != nil {
		return nil, fmt.Errorf("failed to extract database from tar: %w", err)
	}

	// Verify the extracted database
	dbInfo, err := os.Stat(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat extracted database: %w", err)
	}
	if dbInfo.Size() == 0 {
		return nil, fmt.Errorf("extracted database is empty")
	}

	// Copy storage files from container
	// First list what files exist in storage
	exitCode, listOutput, _ := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("find %s -type f 2>/dev/null", containerStoragePath)})
	if exitCode == 0 {
		fileList := strings.TrimSpace(readOutput(listOutput))
		// Remove docker control characters
		fileList = strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' {
				return -1
			}
			return r
		}, fileList)
		
		if fileList != "" {
			fileCount := strings.Count(fileList, "\n") + 1
			fmt.Printf("Storage files in container: %d files\n", fileCount)
			
			// Create tar of storage directory inside container
			const storageTarPath = "/tmp/storage.tar"
			exitCode, _, _ := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf(
				"cd %s && tar -cf %s .",
				containerStoragePath, storageTarPath,
			)})
			if exitCode == 0 {
				// Copy the tar file from container
				// CopyFileFromContainer returns the tar file content directly as a tar stream
				// (not wrapped in another tar) - this is the actual storage.tar we created
				tarReader, tarErr := container.CopyFileFromContainer(ctx, storageTarPath)
				if tarErr != nil {
					fmt.Printf("Warning: Failed to copy storage tar: %v\n", tarErr)
				} else {
					tarData, readErr := io.ReadAll(tarReader)
					tarReader.Close()
					
					if readErr != nil {
						fmt.Printf("Warning: Failed to read storage tar: %v\n", readErr)
					} else if len(tarData) > 0 {
						// The tarData IS the storage.tar content directly
						// Extract the storage contents
						if extractErr := extractTarDirectoryNoStrip(bytes.NewReader(tarData), storagePath); extractErr != nil {
							fmt.Printf("Warning: Failed to extract storage contents: %v\n", extractErr)
						} else {
							// Count extracted files
							var extractedCount int
							filepath.Walk(storagePath, func(path string, info os.FileInfo, err error) error {
								if err == nil && !info.IsDir() {
									extractedCount++
								}
								return nil
							})
							fmt.Printf("Extracted %d storage files\n", extractedCount)
						}
					}
				}
			}
		}
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

// extractTarDirectoryNoStrip extracts all files from a tar stream to destDir
// Unlike extractTarDirectory, this doesn't strip any root directory
func extractTarDirectoryNoStrip(reader io.Reader, destDir string) error {
	// Read all data first to handle potential padding issues
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read tar data: %w", err)
	}
	
	// Tar files are padded to 512-byte blocks, and may have trailing zeros
	// Find the actual content by looking for the tar header magic
	tr := tar.NewReader(bytes.NewReader(data))
	var extractedCount int

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// If we've already extracted some files, don't fail on trailing garbage
			if extractedCount > 0 {
				break
			}
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip the current directory entry
		if header.Name == "." || header.Name == "./" {
			continue
		}

		// Clean the path
		relPath := strings.TrimPrefix(header.Name, "./")
		if relPath == "" {
			continue
		}

		targetPath := filepath.Join(destDir, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg, 0:
			// Ensure parent directory exists
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
			}

			// Read and write file content
			fileContent, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("failed to read file %s from tar: %w", header.Name, err)
			}

			if err := os.WriteFile(targetPath, fileContent, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			extractedCount++
		}
	}

	return nil
}
// extractTarFile extracts a single file from a tar stream and writes it to destPath
func extractTarFile(reader io.Reader, destPath string) error {
	// Read all data from the reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("received empty data from container")
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
			// Read the file content from the tar
			fileContent, readErr := io.ReadAll(tr)
			if readErr != nil {
				return fmt.Errorf("failed to read file from tar: %w", readErr)
			}

			if len(fileContent) == 0 {
				return fmt.Errorf("extracted empty file from tar (header size: %d)", header.Size)
			}

			if err := os.WriteFile(destPath, fileContent, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("no regular file found in tar archive (data size: %d bytes)", len(data))
}
