package selfhost

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ozanturksever/convex-bundler/pkg/manifest"
)

// CreateOptions contains options for creating a self-extracting executable.
type CreateOptions struct {
	// BundleDir is the path to the convex-bundler output directory
	BundleDir string

	// OpsBinary is the path to the convex-backend-ops binary
	OpsBinary string

	// OutputPath is the output path for the self-extracting executable
	OutputPath string

	// Platform is the target platform (e.g., "linux-x64", "linux-arm64")
	Platform string

	// Compression is the compression algorithm ("gzip" or "zstd")
	// Defaults to "gzip" if empty
	Compression string

	// OpsVersion is the version of the ops binary (optional, for metadata)
	OpsVersion string
}

// Create assembles a self-extracting executable from a bundle directory and ops binary.
func Create(opts CreateOptions) error {
	// Set defaults
	if opts.Compression == "" {
		opts.Compression = CompressionGzip
	}

	// Validate inputs
	if err := validateCreateInputs(opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Read manifest from bundle
	manifestPath := filepath.Join(opts.BundleDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var mf manifest.Manifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		return fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	// Create compressed tar archive of bundle
	var compressedBuf bytes.Buffer
	uncompressedSize, err := createCompressedTar(&compressedBuf, opts.BundleDir, opts.Compression)
	if err != nil {
		return fmt.Errorf("failed to create compressed archive: %w", err)
	}

	compressedData := compressedBuf.Bytes()

	// Calculate checksum of compressed data
	checksum := calculateChecksum(compressedData)

	// Build header
	header := NewHeader()
	header.Compression = opts.Compression
	header.BundleSize = uncompressedSize
	header.BundleChecksum = checksum
	header.Manifest = &mf
	header.OpsVersion = opts.OpsVersion
	header.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Validate header
	if err := header.Validate(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// Create output file
	outFile, err := os.Create(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Copy ops binary as base
	opsFile, err := os.Open(opts.OpsBinary)
	if err != nil {
		return fmt.Errorf("failed to open ops binary: %w", err)
	}
	defer opsFile.Close()

	opsStat, err := opsFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat ops binary: %w", err)
	}

	_, err = io.Copy(outFile, opsFile)
	if err != nil {
		return fmt.Errorf("failed to copy ops binary: %w", err)
	}

	// Record the offset where the bundle section starts
	bundleStartOffset := opsStat.Size()

	// Write start marker
	if _, err := outFile.Write(MagicStart); err != nil {
		return fmt.Errorf("failed to write start marker: %w", err)
	}

	// Write length-prefixed header
	if _, err := WriteHeader(outFile, header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write compressed bundle
	if _, err := outFile.Write(compressedData); err != nil {
		return fmt.Errorf("failed to write compressed bundle: %w", err)
	}

	// Write end marker
	if _, err := outFile.Write(MagicEnd); err != nil {
		return fmt.Errorf("failed to write end marker: %w", err)
	}

	// Write footer (offset to start marker as uint64 little-endian)
	footer := make([]byte, FooterSize)
	binary.LittleEndian.PutUint64(footer, uint64(bundleStartOffset))
	if _, err := outFile.Write(footer); err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}

	// Make executable
	if err := outFile.Chmod(0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	return nil
}

// DetectResult contains the result of self-host detection.
type DetectResult struct {
	// IsSelfHost indicates whether the executable contains an embedded bundle
	IsSelfHost bool

	// Offset is the byte offset where the bundle section starts (at MagicStart)
	Offset int64
}

// DetectSelfHostMode checks if the current executable contains an embedded bundle.
// It reads the footer to find the offset and verifies the magic marker.
func DetectSelfHostMode() (*DetectResult, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	return DetectSelfHostModeFromFile(exePath)
}

// DetectSelfHostModeFromFile checks if the given file contains an embedded bundle.
func DetectSelfHostModeFromFile(path string) (*DetectResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()

	// File must be large enough to contain at least the footer
	if fileSize < FooterSize {
		return &DetectResult{IsSelfHost: false}, nil
	}

	// Read footer (last 8 bytes)
	if _, err := f.Seek(-FooterSize, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	footer := make([]byte, FooterSize)
	if _, err := io.ReadFull(f, footer); err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	offset := int64(binary.LittleEndian.Uint64(footer))

	// Sanity check: offset must be within file bounds
	if offset < 0 || offset >= fileSize-FooterSize {
		return &DetectResult{IsSelfHost: false}, nil
	}

	// Seek to offset and check for magic marker
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to bundle start: %w", err)
	}

	marker := make([]byte, MagicStartLen)
	if _, err := io.ReadFull(f, marker); err != nil {
		return &DetectResult{IsSelfHost: false}, nil
	}

	if !bytes.Equal(marker, MagicStart) {
		return &DetectResult{IsSelfHost: false}, nil
	}

	return &DetectResult{
		IsSelfHost: true,
		Offset:     offset,
	}, nil
}

// ReadHeaderFromExecutable reads the header from a self-extracting executable.
// If path is empty, uses the current executable.
func ReadHeaderFromExecutable(path string) (*Header, error) {
	if path == "" {
		var err error
		path, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	result, err := DetectSelfHostModeFromFile(path)
	if err != nil {
		return nil, err
	}

	if !result.IsSelfHost {
		return nil, fmt.Errorf("file is not a self-host executable")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Seek past the start marker to the header
	if _, err := f.Seek(result.Offset+MagicStartLen, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to header: %w", err)
	}

	return ReadHeader(f)
}

// ExtractOptions contains options for extracting an embedded bundle.
type ExtractOptions struct {
	// ExecutablePath is the path to the self-extracting executable.
	// If empty, uses the current executable.
	ExecutablePath string

	// OutputDir is the directory to extract the bundle to.
	OutputDir string

	// SkipVerify skips checksum verification if true.
	SkipVerify bool
}

// Extract extracts the embedded bundle from a self-extracting executable.
func Extract(opts ExtractOptions) (*Header, error) {
	exePath := opts.ExecutablePath
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	// Detect self-host mode
	result, err := DetectSelfHostModeFromFile(exePath)
	if err != nil {
		return nil, err
	}

	if !result.IsSelfHost {
		return nil, fmt.Errorf("file does not contain an embedded bundle")
	}

	f, err := os.Open(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open executable: %w", err)
	}
	defer f.Close()

	// Seek past start marker to header
	if _, err := f.Seek(result.Offset+MagicStartLen, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to header: %w", err)
	}

	// Read header
	header, err := ReadHeader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Current position is at the start of compressed data
	compressedDataStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate compressed data size:
	// file size - compressed start - end marker - footer
	compressedDataSize := stat.Size() - compressedDataStart - MagicEndLen - FooterSize

	// Read compressed data for verification
	compressedData := make([]byte, compressedDataSize)
	if _, err := io.ReadFull(f, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Verify checksum if not skipped
	if !opts.SkipVerify {
		calculatedChecksum := calculateChecksum(compressedData)
		if calculatedChecksum != header.BundleChecksum {
			return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", header.BundleChecksum, calculatedChecksum)
		}
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Decompress and extract
	if err := extractCompressedTar(compressedData, opts.OutputDir, header.Compression); err != nil {
		return nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	return header, nil
}

// VerifyResult contains the result of bundle verification.
type VerifyResult struct {
	// Valid indicates whether the checksum matched
	Valid bool

	// ExpectedChecksum is the checksum stored in the header
	ExpectedChecksum string

	// ActualChecksum is the calculated checksum
	ActualChecksum string
}

// Verify verifies the integrity of the embedded bundle.
func Verify(path string) (*VerifyResult, error) {
	if path == "" {
		var err error
		path, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	// Detect self-host mode
	result, err := DetectSelfHostModeFromFile(path)
	if err != nil {
		return nil, err
	}

	if !result.IsSelfHost {
		return nil, fmt.Errorf("file does not contain an embedded bundle")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Seek past start marker to header
	if _, err := f.Seek(result.Offset+MagicStartLen, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to header: %w", err)
	}

	// Read header
	header, err := ReadHeader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Current position is at compressed data
	compressedDataStart, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current position: %w", err)
	}

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate compressed data size
	compressedDataSize := stat.Size() - compressedDataStart - MagicEndLen - FooterSize

	// Read compressed data
	compressedData := make([]byte, compressedDataSize)
	if _, err := io.ReadFull(f, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Calculate checksum
	actualChecksum := calculateChecksum(compressedData)

	return &VerifyResult{
		Valid:            actualChecksum == header.BundleChecksum,
		ExpectedChecksum: header.BundleChecksum,
		ActualChecksum:   actualChecksum,
	}, nil
}

// CheckPlatformCompatibility checks if the bundle platform matches the host.
func CheckPlatformCompatibility(bundlePlatform string) error {
	hostPlatform := getHostPlatform()

	if bundlePlatform != hostPlatform {
		return fmt.Errorf("platform mismatch: bundle is for %s, host is %s", bundlePlatform, hostPlatform)
	}

	return nil
}

// getHostPlatform returns the current host platform in the format used by bundles.
func getHostPlatform() string {
	platformMap := map[string]string{
		"linux-amd64": "linux-x64",
		"linux-arm64": "linux-arm64",
		"darwin-amd64": "darwin-x64",
		"darwin-arm64": "darwin-arm64",
	}

	key := runtime.GOOS + "-" + runtime.GOARCH
	if platform, ok := platformMap[key]; ok {
		return platform
	}

	// Return as-is if not in map
	return key
}

// validateCreateInputs validates the inputs for Create.
func validateCreateInputs(opts CreateOptions) error {
	if opts.BundleDir == "" {
		return fmt.Errorf("bundle directory is required")
	}

	if opts.OpsBinary == "" {
		return fmt.Errorf("ops binary is required")
	}

	if opts.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}

	if opts.Platform == "" {
		return fmt.Errorf("platform is required")
	}

	// Check bundle directory exists
	info, err := os.Stat(opts.BundleDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("bundle directory does not exist: %s", opts.BundleDir)
	}
	if err != nil {
		return fmt.Errorf("failed to access bundle directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("bundle path is not a directory: %s", opts.BundleDir)
	}

	// Check required bundle files exist
	requiredFiles := []string{"manifest.json", "backend", "convex.db", "credentials.json"}
	for _, file := range requiredFiles {
		path := filepath.Join(opts.BundleDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("bundle is missing required file: %s", file)
		}
	}

	// Check ops binary exists
	info, err = os.Stat(opts.OpsBinary)
	if os.IsNotExist(err) {
		return fmt.Errorf("ops binary does not exist: %s", opts.OpsBinary)
	}
	if err != nil {
		return fmt.Errorf("failed to access ops binary: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("ops binary path is a directory: %s", opts.OpsBinary)
	}

	// Validate compression
	if opts.Compression != CompressionGzip && opts.Compression != CompressionZstd && opts.Compression != "" {
		return fmt.Errorf("invalid compression: %s (must be %q or %q)", opts.Compression, CompressionGzip, CompressionZstd)
	}

	return nil
}

// createCompressedTar creates a compressed tar archive of the bundle directory.
// Returns the uncompressed size.
func createCompressedTar(w io.Writer, bundleDir string, compression string) (int64, error) {
	var compressWriter io.WriteCloser
	var err error

	switch compression {
	case CompressionGzip, "":
		compressWriter = gzip.NewWriter(w)
	case CompressionZstd:
		// For now, we only support gzip. Zstd would require an additional dependency.
		return 0, fmt.Errorf("zstd compression is not yet implemented")
	default:
		return 0, fmt.Errorf("unsupported compression: %s", compression)
	}
	defer compressWriter.Close()

	tarWriter := tar.NewWriter(compressWriter)
	defer tarWriter.Close()

	var totalSize int64

	err = filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(bundleDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", relPath, err)
		}

		// Use relative path as the name
		header.Name = relPath

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			header.Linkname = link
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", relPath, err)
		}

		// Write file content (skip directories)
		if !info.IsDir() && info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", path, err)
			}
			defer file.Close()

			n, err := io.Copy(tarWriter, file)
			if err != nil {
				return fmt.Errorf("failed to write %s to tar: %w", relPath, err)
			}
			totalSize += n
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

// extractCompressedTar extracts a compressed tar archive to the output directory.
func extractCompressedTar(compressedData []byte, outputDir string, compression string) error {
	reader := bytes.NewReader(compressedData)

	var decompressReader io.ReadCloser
	var err error

	switch compression {
	case CompressionGzip, "":
		decompressReader, err = gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
	case CompressionZstd:
		return fmt.Errorf("zstd decompression is not yet implemented")
	default:
		return fmt.Errorf("unsupported compression: %s", compression)
	}
	defer decompressReader.Close()

	tarReader := tar.NewReader(decompressReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Sanitize the path to prevent path traversal attacks
		targetPath := filepath.Join(outputDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(outputDir)) {
			return fmt.Errorf("invalid path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			file.Close()

		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for symlink %s: %w", targetPath, err)
			}

			// Remove existing file/symlink if it exists
			os.Remove(targetPath)

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}

		default:
			// Skip other types (devices, etc.)
			continue
		}
	}

	return nil
}

// calculateChecksum calculates the SHA256 checksum of data.
// Returns the checksum in the format "sha256:hexstring".
func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}
