package selfhost

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ozanturksever/convex-bundler/pkg/manifest"
)

// Helper function to create a mock bundle directory with all required files
func createMockBundleDir(t *testing.T, dir string) {
	t.Helper()

	// Create manifest.json
	mf := manifest.New(manifest.Options{
		Name:     "Test Bundle",
		Version:  "1.0.0",
		Apps:     []string{"./app1"},
		Platform: "linux-x64",
	})
	manifestData, err := mf.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), manifestData, 0644))

	// Create mock backend binary
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backend"), []byte("#!/bin/bash\necho 'mock backend'"), 0755))

	// Create mock database
	require.NoError(t, os.WriteFile(filepath.Join(dir, "convex.db"), []byte("mock database content"), 0644))

	// Create mock credentials
	credsData := `{"adminKey": "test-admin-key", "instanceSecret": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(credsData), 0644))

	// Create storage directory
	storageDir := filepath.Join(dir, "storage")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	// Add a test file in storage
	require.NoError(t, os.WriteFile(filepath.Join(storageDir, "test-file.txt"), []byte("test storage content"), 0644))
}

// Helper function to create a mock ops binary
func createMockOpsBinary(t *testing.T, path string) {
	t.Helper()
	// Create a simple shell script as mock ops binary
	content := `#!/bin/bash
echo "mock convex-backend-ops"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0755))
}

// TestHeaderSerialization tests JSON round-trip for header
func TestHeaderSerialization(t *testing.T) {
	mf := manifest.New(manifest.Options{
		Name:     "Test App",
		Version:  "2.0.0",
		Apps:     []string{"./app1", "./app2"},
		Platform: "linux-arm64",
	})

	original := &Header{
		Version:        HeaderVersion,
		Format:         HeaderFormat,
		Compression:    CompressionGzip,
		BundleSize:     12345678,
		BundleChecksum: "sha256:abcdef1234567890",
		Manifest:       mf,
		OpsVersion:     "1.5.0",
		CreatedAt:      "2024-01-15T10:30:00Z",
	}

	// Serialize
	data, err := original.ToJSON()
	require.NoError(t, err)

	// Deserialize
	parsed := &Header{}
	err = parsed.FromJSON(data)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, original.Version, parsed.Version)
	assert.Equal(t, original.Format, parsed.Format)
	assert.Equal(t, original.Compression, parsed.Compression)
	assert.Equal(t, original.BundleSize, parsed.BundleSize)
	assert.Equal(t, original.BundleChecksum, parsed.BundleChecksum)
	assert.Equal(t, original.OpsVersion, parsed.OpsVersion)
	assert.Equal(t, original.CreatedAt, parsed.CreatedAt)
	assert.Equal(t, original.Manifest.Name, parsed.Manifest.Name)
	assert.Equal(t, original.Manifest.Version, parsed.Manifest.Version)
	assert.Equal(t, original.Manifest.Platform, parsed.Manifest.Platform)
}

// TestHeaderValidation tests header validation
func TestHeaderValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Header)
		wantErr string
	}{
		{
			name:    "valid header",
			modify:  func(h *Header) {},
			wantErr: "",
		},
		{
			name:    "missing version",
			modify:  func(h *Header) { h.Version = "" },
			wantErr: "header version is required",
		},
		{
			name:    "invalid format",
			modify:  func(h *Header) { h.Format = "invalid" },
			wantErr: "invalid header format",
		},
		{
			name:    "invalid compression",
			modify:  func(h *Header) { h.Compression = "lz4" },
			wantErr: "invalid compression",
		},
		{
			name:    "zero bundle size",
			modify:  func(h *Header) { h.BundleSize = 0 },
			wantErr: "bundle size must be positive",
		},
		{
			name:    "negative bundle size",
			modify:  func(h *Header) { h.BundleSize = -1 },
			wantErr: "bundle size must be positive",
		},
		{
			name:    "missing checksum",
			modify:  func(h *Header) { h.BundleChecksum = "" },
			wantErr: "bundle checksum is required",
		},
		{
			name:    "missing manifest",
			modify:  func(h *Header) { h.Manifest = nil },
			wantErr: "manifest is required",
		},
		{
			name:    "missing createdAt",
			modify:  func(h *Header) { h.CreatedAt = "" },
			wantErr: "createdAt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mf := manifest.New(manifest.Options{
				Name:     "Test",
				Version:  "1.0.0",
				Apps:     []string{"./app"},
				Platform: "linux-x64",
			})

			header := &Header{
				Version:        HeaderVersion,
				Format:         HeaderFormat,
				Compression:    CompressionGzip,
				BundleSize:     1000,
				BundleChecksum: "sha256:abc123",
				Manifest:       mf,
				OpsVersion:     "1.0.0",
				CreatedAt:      "2024-01-15T10:30:00Z",
			}

			tt.modify(header)

			err := header.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestWriteReadHeader tests header write and read with length prefix
func TestWriteReadHeader(t *testing.T) {
	mf := manifest.New(manifest.Options{
		Name:     "Test",
		Version:  "1.0.0",
		Apps:     []string{"./app"},
		Platform: "linux-x64",
	})

	original := &Header{
		Version:        HeaderVersion,
		Format:         HeaderFormat,
		Compression:    CompressionGzip,
		BundleSize:     999999,
		BundleChecksum: "sha256:deadbeef",
		Manifest:       mf,
		OpsVersion:     "2.0.0",
		CreatedAt:      "2024-06-01T12:00:00Z",
	}

	// Write to buffer
	var buf bytes.Buffer
	n, err := WriteHeader(&buf, original)
	require.NoError(t, err)
	assert.Greater(t, n, HeaderLengthSize)

	// Read back
	parsed, err := ReadHeader(&buf)
	require.NoError(t, err)

	assert.Equal(t, original.Version, parsed.Version)
	assert.Equal(t, original.Format, parsed.Format)
	assert.Equal(t, original.BundleSize, parsed.BundleSize)
	assert.Equal(t, original.BundleChecksum, parsed.BundleChecksum)
}

// TestCreate_ValidBundle tests creating a self-extracting executable from a valid bundle
func TestCreate_ValidBundle(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock bundle directory
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	// Create mock ops binary
	opsBinary := filepath.Join(tmpDir, "ops-binary")
	createMockOpsBinary(t, opsBinary)

	// Output path
	outputPath := filepath.Join(tmpDir, "selfhost-executable")

	// Create self-extracting executable
	err := Create(CreateOptions{
		BundleDir:   bundleDir,
		OpsBinary:   opsBinary,
		OutputPath:  outputPath,
		Platform:    "linux-x64",
		Compression: CompressionGzip,
		OpsVersion:  "1.0.0",
	})
	require.NoError(t, err)

	// Verify output file exists
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "output should be executable")

	// Verify file is larger than ops binary (contains embedded bundle)
	opsInfo, err := os.Stat(opsBinary)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), opsInfo.Size(), "self-host executable should be larger than ops binary")

	// Verify it's detected as self-host
	result, err := DetectSelfHostModeFromFile(outputPath)
	require.NoError(t, err)
	assert.True(t, result.IsSelfHost)
	assert.Greater(t, result.Offset, int64(0))
}

// TestDetectSelfHostMode_RegularBinary tests that a regular binary is not detected as self-host
func TestDetectSelfHostMode_RegularBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular binary (not self-host)
	regularBinary := filepath.Join(tmpDir, "regular-binary")
	content := `#!/bin/bash
echo "I am a regular binary"
`
	require.NoError(t, os.WriteFile(regularBinary, []byte(content), 0755))

	result, err := DetectSelfHostModeFromFile(regularBinary)
	require.NoError(t, err)
	assert.False(t, result.IsSelfHost)
}

// TestDetectSelfHostMode_SelfHostBinary tests that a self-host binary is correctly detected
func TestDetectSelfHostMode_SelfHostBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a full self-host executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	outputPath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: outputPath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	result, err := DetectSelfHostModeFromFile(outputPath)
	require.NoError(t, err)
	assert.True(t, result.IsSelfHost)
}

// TestExtract_ValidExecutable tests extracting a bundle from a self-extracting executable
func TestExtract_ValidExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock bundle
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	// Create self-extracting executable
	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Extract to new directory
	extractDir := filepath.Join(tmpDir, "extracted")
	header, err := Extract(ExtractOptions{
		ExecutablePath: executablePath,
		OutputDir:      extractDir,
	})
	require.NoError(t, err)

	// Verify header
	assert.Equal(t, "Test Bundle", header.Manifest.Name)
	assert.Equal(t, "1.0.0", header.Manifest.Version)
	assert.Equal(t, "linux-x64", header.Manifest.Platform)

	// Verify extracted files
	assertExtractedBundleStructure(t, extractDir)

	// Verify file contents match
	originalBackend, err := os.ReadFile(filepath.Join(bundleDir, "backend"))
	require.NoError(t, err)
	extractedBackend, err := os.ReadFile(filepath.Join(extractDir, "backend"))
	require.NoError(t, err)
	assert.Equal(t, originalBackend, extractedBackend)

	originalDB, err := os.ReadFile(filepath.Join(bundleDir, "convex.db"))
	require.NoError(t, err)
	extractedDB, err := os.ReadFile(filepath.Join(extractDir, "convex.db"))
	require.NoError(t, err)
	assert.Equal(t, originalDB, extractedDB)

	// Verify storage file
	originalStorage, err := os.ReadFile(filepath.Join(bundleDir, "storage", "test-file.txt"))
	require.NoError(t, err)
	extractedStorage, err := os.ReadFile(filepath.Join(extractDir, "storage", "test-file.txt"))
	require.NoError(t, err)
	assert.Equal(t, originalStorage, extractedStorage)
}

// TestVerify_ChecksumMatch tests that verification passes for a valid executable
func TestVerify_ChecksumMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create self-extracting executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Verify
	result, err := Verify(executablePath)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, result.ExpectedChecksum, result.ActualChecksum)
}

// TestVerify_ChecksumMismatch tests that verification fails for a corrupted executable
func TestVerify_ChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create self-extracting executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Corrupt the file by modifying bytes in the middle (in the compressed data section)
	data, err := os.ReadFile(executablePath)
	require.NoError(t, err)

	// Find approximately where the compressed data is and corrupt it
	// The compressed data is after: ops binary + magic start (20) + header length (4) + header json
	// We'll corrupt some bytes towards the end but before the footer
	corruptionOffset := len(data) - 100
	if corruptionOffset > 0 {
		data[corruptionOffset] ^= 0xFF
		data[corruptionOffset+1] ^= 0xFF
		data[corruptionOffset+2] ^= 0xFF
	}

	err = os.WriteFile(executablePath, data, 0755)
	require.NoError(t, err)

	// Verify should fail
	result, err := Verify(executablePath)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.NotEqual(t, result.ExpectedChecksum, result.ActualChecksum)
}

// TestReadHeaderFromExecutable tests reading header from self-extracting executable
func TestReadHeaderFromExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create self-extracting executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
		OpsVersion: "1.5.0",
	})
	require.NoError(t, err)

	// Read header
	header, err := ReadHeaderFromExecutable(executablePath)
	require.NoError(t, err)

	assert.Equal(t, HeaderVersion, header.Version)
	assert.Equal(t, HeaderFormat, header.Format)
	assert.Equal(t, CompressionGzip, header.Compression)
	assert.Equal(t, "1.5.0", header.OpsVersion)
	assert.Equal(t, "Test Bundle", header.Manifest.Name)
	assert.Equal(t, "1.0.0", header.Manifest.Version)
	assert.Equal(t, "linux-x64", header.Manifest.Platform)
	assert.NotEmpty(t, header.CreatedAt)
	assert.NotEmpty(t, header.BundleChecksum)
	assert.Greater(t, header.BundleSize, int64(0))
}

// TestReadHeaderFromExecutable_NotSelfHost tests error when reading header from non-selfhost file
func TestReadHeaderFromExecutable_NotSelfHost(t *testing.T) {
	tmpDir := t.TempDir()

	regularFile := filepath.Join(tmpDir, "regular")
	require.NoError(t, os.WriteFile(regularFile, []byte("not a selfhost file"), 0644))

	_, err := ReadHeaderFromExecutable(regularFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a self-host executable")
}

// TestValidateCreateInputs tests validation of create options
func TestValidateCreateInputs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid bundle and ops binary for reference
	validBundleDir := filepath.Join(tmpDir, "valid-bundle")
	require.NoError(t, os.MkdirAll(validBundleDir, 0755))
	createMockBundleDir(t, validBundleDir)

	validOpsBinary := filepath.Join(tmpDir, "valid-ops")
	createMockOpsBinary(t, validOpsBinary)

	tests := []struct {
		name    string
		opts    CreateOptions
		wantErr string
	}{
		{
			name: "valid options",
			opts: CreateOptions{
				BundleDir:  validBundleDir,
				OpsBinary:  validOpsBinary,
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			},
			wantErr: "",
		},
		{
			name: "missing bundle dir",
			opts: CreateOptions{
				BundleDir:  "",
				OpsBinary:  validOpsBinary,
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			},
			wantErr: "bundle directory is required",
		},
		{
			name: "missing ops binary",
			opts: CreateOptions{
				BundleDir:  validBundleDir,
				OpsBinary:  "",
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			},
			wantErr: "ops binary is required",
		},
		{
			name: "missing output path",
			opts: CreateOptions{
				BundleDir:  validBundleDir,
				OpsBinary:  validOpsBinary,
				OutputPath: "",
				Platform:   "linux-x64",
			},
			wantErr: "output path is required",
		},
		{
			name: "missing platform",
			opts: CreateOptions{
				BundleDir:  validBundleDir,
				OpsBinary:  validOpsBinary,
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "",
			},
			wantErr: "platform is required",
		},
		{
			name: "bundle dir does not exist",
			opts: CreateOptions{
				BundleDir:  filepath.Join(tmpDir, "nonexistent"),
				OpsBinary:  validOpsBinary,
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			},
			wantErr: "bundle directory does not exist",
		},
		{
			name: "ops binary does not exist",
			opts: CreateOptions{
				BundleDir:  validBundleDir,
				OpsBinary:  filepath.Join(tmpDir, "nonexistent-ops"),
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			},
			wantErr: "ops binary does not exist",
		},
		{
			name: "invalid compression",
			opts: CreateOptions{
				BundleDir:   validBundleDir,
				OpsBinary:   validOpsBinary,
				OutputPath:  filepath.Join(tmpDir, "output"),
				Platform:    "linux-x64",
				Compression: "lz4",
			},
			wantErr: "invalid compression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateInputs(tt.opts)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestValidateCreateInputs_MissingBundleFiles tests validation when bundle is missing required files
func TestValidateCreateInputs_MissingBundleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	tests := []struct {
		name        string
		setupBundle func(string)
		wantErr     string
	}{
		{
			name: "missing manifest.json",
			setupBundle: func(dir string) {
				os.WriteFile(filepath.Join(dir, "backend"), []byte("x"), 0755)
				os.WriteFile(filepath.Join(dir, "convex.db"), []byte("x"), 0644)
				os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{}"), 0644)
			},
			wantErr: "missing required file: manifest.json",
		},
		{
			name: "missing backend",
			setupBundle: func(dir string) {
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "convex.db"), []byte("x"), 0644)
				os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{}"), 0644)
			},
			wantErr: "missing required file: backend",
		},
		{
			name: "missing convex.db",
			setupBundle: func(dir string) {
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "backend"), []byte("x"), 0755)
				os.WriteFile(filepath.Join(dir, "credentials.json"), []byte("{}"), 0644)
			},
			wantErr: "missing required file: convex.db",
		},
		{
			name: "missing credentials.json",
			setupBundle: func(dir string) {
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "backend"), []byte("x"), 0755)
				os.WriteFile(filepath.Join(dir, "convex.db"), []byte("x"), 0644)
			},
			wantErr: "missing required file: credentials.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundleDir := filepath.Join(tmpDir, tt.name)
			require.NoError(t, os.MkdirAll(bundleDir, 0755))
			tt.setupBundle(bundleDir)

			err := validateCreateInputs(CreateOptions{
				BundleDir:  bundleDir,
				OpsBinary:  opsBinary,
				OutputPath: filepath.Join(tmpDir, "output"),
				Platform:   "linux-x64",
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPlatformCompatibility tests platform matching
func TestPlatformCompatibility(t *testing.T) {
	// Note: These tests check the logic, actual platform depends on runtime.GOOS/GOARCH
	// We can't easily test all cases, but we test the function works

	// Get current host platform
	hostPlatform := getHostPlatform()
	assert.NotEmpty(t, hostPlatform)

	// Matching platform should succeed
	err := CheckPlatformCompatibility(hostPlatform)
	assert.NoError(t, err)

	// Mismatching platform should fail
	wrongPlatform := "nonexistent-platform"
	err = CheckPlatformCompatibility(wrongPlatform)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform mismatch")
}

// TestMagicMarkerLengths verifies magic marker constants have correct lengths
func TestMagicMarkerLengths(t *testing.T) {
	assert.Equal(t, MagicStartLen, len(MagicStart), "MagicStart should be %d bytes", MagicStartLen)
	assert.Equal(t, MagicEndLen, len(MagicEnd), "MagicEnd should be %d bytes", MagicEndLen)
}

// TestCalculateChecksum tests checksum calculation
func TestCalculateChecksum(t *testing.T) {
	data := []byte("test data for checksum")
	checksum := calculateChecksum(data)

	assert.True(t, len(checksum) > 7, "checksum should have sha256: prefix")
	assert.Equal(t, "sha256:", checksum[:7])

	// Same data should produce same checksum
	checksum2 := calculateChecksum(data)
	assert.Equal(t, checksum, checksum2)

	// Different data should produce different checksum
	differentData := []byte("different data")
	differentChecksum := calculateChecksum(differentData)
	assert.NotEqual(t, checksum, differentChecksum)
}

// TestRoundTrip tests the full create -> extract -> verify cycle
func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create original bundle
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	// Add some additional files to test thorough extraction
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "extra-file.txt"), []byte("extra content"), 0644))
	nestedDir := filepath.Join(bundleDir, "storage", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "deep-file.txt"), []byte("deep content"), 0644))

	// Create ops binary
	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	// Create self-extracting executable
	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
		OpsVersion: "1.2.3",
	})
	require.NoError(t, err)

	// Verify the executable
	verifyResult, err := Verify(executablePath)
	require.NoError(t, err)
	assert.True(t, verifyResult.Valid, "verification should pass")

	// Read header to check metadata
	header, err := ReadHeaderFromExecutable(executablePath)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", header.OpsVersion)

	// Extract the bundle
	extractDir := filepath.Join(tmpDir, "extracted")
	_, err = Extract(ExtractOptions{
		ExecutablePath: executablePath,
		OutputDir:      extractDir,
	})
	require.NoError(t, err)

	// Verify all files match
	verifyFilesMatch(t, bundleDir, extractDir, "")
}

// Helper functions

func assertExtractedBundleStructure(t *testing.T, dir string) {
	t.Helper()

	// Check backend
	backendPath := filepath.Join(dir, "backend")
	info, err := os.Stat(backendPath)
	require.NoError(t, err, "backend should exist")
	assert.True(t, info.Mode()&0111 != 0, "backend should be executable")

	// Check convex.db
	_, err = os.Stat(filepath.Join(dir, "convex.db"))
	require.NoError(t, err, "convex.db should exist")

	// Check manifest.json
	_, err = os.Stat(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err, "manifest.json should exist")

	// Check credentials.json
	_, err = os.Stat(filepath.Join(dir, "credentials.json"))
	require.NoError(t, err, "credentials.json should exist")

	// Check storage directory
	info, err = os.Stat(filepath.Join(dir, "storage"))
	require.NoError(t, err, "storage directory should exist")
	assert.True(t, info.IsDir(), "storage should be a directory")
}

func verifyFilesMatch(t *testing.T, originalDir, extractedDir, relativePath string) {
	t.Helper()

	originalPath := filepath.Join(originalDir, relativePath)
	extractedPath := filepath.Join(extractedDir, relativePath)

	originalInfo, err := os.Stat(originalPath)
	require.NoError(t, err)

	extractedInfo, err := os.Stat(extractedPath)
	require.NoError(t, err)

	if originalInfo.IsDir() {
		assert.True(t, extractedInfo.IsDir(), "expected directory at %s", relativePath)

		entries, err := os.ReadDir(originalPath)
		require.NoError(t, err)

		for _, entry := range entries {
			childPath := filepath.Join(relativePath, entry.Name())
			verifyFilesMatch(t, originalDir, extractedDir, childPath)
		}
	} else {
		originalContent, err := os.ReadFile(originalPath)
		require.NoError(t, err)

		extractedContent, err := os.ReadFile(extractedPath)
		require.NoError(t, err)

		assert.Equal(t, originalContent, extractedContent, "content mismatch for %s", relativePath)
	}
}

// TestExtract_SkipVerify tests extraction with checksum verification skipped
func TestExtract_SkipVerify(t *testing.T) {
	tmpDir := t.TempDir()

	// Create self-extracting executable
	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))
	createMockBundleDir(t, bundleDir)

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err := Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-x64",
	})
	require.NoError(t, err)

	// Extract with SkipVerify
	extractDir := filepath.Join(tmpDir, "extracted")
	header, err := Extract(ExtractOptions{
		ExecutablePath: executablePath,
		OutputDir:      extractDir,
		SkipVerify:     true,
	})
	require.NoError(t, err)
	assert.NotNil(t, header)

	// Files should still be extracted
	assertExtractedBundleStructure(t, extractDir)
}

// TestExtract_NotSelfHost tests extraction error for non-selfhost files
func TestExtract_NotSelfHost(t *testing.T) {
	tmpDir := t.TempDir()

	regularFile := filepath.Join(tmpDir, "regular")
	require.NoError(t, os.WriteFile(regularFile, []byte("not a selfhost file"), 0644))

	_, err := Extract(ExtractOptions{
		ExecutablePath: regularFile,
		OutputDir:      filepath.Join(tmpDir, "extracted"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain an embedded bundle")
}

// TestVerify_NotSelfHost tests verification error for non-selfhost files
func TestVerify_NotSelfHost(t *testing.T) {
	tmpDir := t.TempDir()

	regularFile := filepath.Join(tmpDir, "regular")
	require.NoError(t, os.WriteFile(regularFile, []byte("not a selfhost file"), 0644))

	_, err := Verify(regularFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain an embedded bundle")
}

// TestManifestParsing tests that manifest is correctly parsed during create
func TestManifestParsing(t *testing.T) {
	tmpDir := t.TempDir()

	bundleDir := filepath.Join(tmpDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	// Create custom manifest
	mf := manifest.New(manifest.Options{
		Name:     "Custom App Name",
		Version:  "3.2.1",
		Apps:     []string{"./app1", "./app2", "./app3"},
		Platform: "linux-arm64",
	})
	manifestData, err := mf.ToJSON()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "manifest.json"), manifestData, 0644))

	// Create other required files
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "backend"), []byte("backend"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "convex.db"), []byte("db"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "credentials.json"), []byte("{}"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "storage"), 0755))

	opsBinary := filepath.Join(tmpDir, "ops")
	createMockOpsBinary(t, opsBinary)

	executablePath := filepath.Join(tmpDir, "selfhost")
	err = Create(CreateOptions{
		BundleDir:  bundleDir,
		OpsBinary:  opsBinary,
		OutputPath: executablePath,
		Platform:   "linux-arm64",
	})
	require.NoError(t, err)

	// Read header and verify manifest
	header, err := ReadHeaderFromExecutable(executablePath)
	require.NoError(t, err)

	assert.Equal(t, "Custom App Name", header.Manifest.Name)
	assert.Equal(t, "3.2.1", header.Manifest.Version)
	assert.Equal(t, "linux-arm64", header.Manifest.Platform)
	assert.Len(t, header.Manifest.Apps, 3)
	assert.Equal(t, []string{"./app1", "./app2", "./app3"}, header.Manifest.Apps)
}

// TestNewHeader tests the NewHeader constructor
func TestNewHeader(t *testing.T) {
	header := NewHeader()

	assert.Equal(t, HeaderVersion, header.Version)
	assert.Equal(t, HeaderFormat, header.Format)
	assert.Equal(t, CompressionGzip, header.Compression)
	assert.Nil(t, header.Manifest)
	assert.Empty(t, header.BundleChecksum)
	assert.Empty(t, header.CreatedAt)
}

// TestExitCodes tests that exit codes have expected values
func TestExitCodes(t *testing.T) {
	assert.Equal(t, 0, ExitSuccess)
	assert.Equal(t, 1, ExitGeneralError)
	assert.Equal(t, 2, ExitInvalidArguments)
	assert.Equal(t, 3, ExitVerificationFailed)
	assert.Equal(t, 4, ExitPlatformMismatch)
	assert.Equal(t, 5, ExitExtractionFailed)
	assert.Equal(t, 6, ExitInstallationFailed)
}

// BenchmarkCreate benchmarks the create operation
func BenchmarkCreate(b *testing.B) {
	tmpDir := b.TempDir()

	bundleDir := filepath.Join(tmpDir, "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		b.Fatal(err)
	}

	// Create manifest
	mf := &manifest.Manifest{
		Name:     "Bench",
		Version:  "1.0.0",
		Apps:     []string{"./app"},
		Platform: "linux-x64",
	}
	manifestData, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(filepath.Join(bundleDir, "manifest.json"), manifestData, 0644)
	os.WriteFile(filepath.Join(bundleDir, "backend"), make([]byte, 1024), 0755)
	os.WriteFile(filepath.Join(bundleDir, "convex.db"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(bundleDir, "credentials.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(bundleDir, "storage"), 0755)

	opsBinary := filepath.Join(tmpDir, "ops")
	os.WriteFile(opsBinary, make([]byte, 1024), 0755)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(tmpDir, "output", string(rune(i)))
		Create(CreateOptions{
			BundleDir:  bundleDir,
			OpsBinary:  opsBinary,
			OutputPath: outputPath,
			Platform:   "linux-x64",
		})
	}
}
