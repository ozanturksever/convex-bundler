# convex-backend-selfhost

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Self-extracting single-file deployment tool for Convex backend with pre-deployed apps. Combines a [convex-bundler](https://github.com/ozanturksever/convex-bundler) bundle with [convex-backend-ops](https://github.com/ozanturksever/convex-backend-ops) into a portable executable.

## Overview

`convex-backend-selfhost` is a single executable file that contains:

1. **convex-backend-ops** - The operations tool for managing Convex backend
2. **Embedded Bundle** - A complete convex-bundler bundle with:
   - `backend` - The convex-local-backend binary
   - `convex.db` - Pre-initialized SQLite database with deployed apps
   - `storage/` - File storage directory
   - `manifest.json` - Bundle metadata
   - `credentials.json` - Pre-generated admin credentials

When executed, the single file can:
- Extract and install the complete Convex backend
- Manage upgrades, rollbacks, and operations
- Be distributed as a single portable file

## Features

- **Single File Distribution** - One executable contains everything needed
- **Self-Extracting** - No external tools needed to deploy
- **Air-Gap Friendly** - Perfect for restricted network environments
- **Embedded Operations** - Full convex-backend-ops functionality included
- **Platform Native** - Separate builds for linux-x64, linux-arm64

---

## File Format

### Structure

The self-extracting executable uses an append-only archive format:

```
┌─────────────────────────────────────────┐
│  convex-backend-ops binary (ELF/Mach-O) │  <- Executable header
├─────────────────────────────────────────┤
│  Magic Marker: "CONVEX_BUNDLE_START"    │  <- 20 bytes
├─────────────────────────────────────────┤
│  Header (JSON, length-prefixed)         │  <- Archive metadata
├─────────────────────────────────────────┤
│  Compressed Bundle (tar.gz)             │  <- Bundle payload
├─────────────────────────────────────────┤
│  Magic Marker: "CONVEX_BUNDLE_END"      │  <- 18 bytes
├─────────────────────────────────────────┤
│  Footer: offset to CONVEX_BUNDLE_START  │  <- 8 bytes (uint64 LE)
└─────────────────────────────────────────┘
```

### Header Format

```json
{
  "version": "1.0.0",
  "format": "selfhost-v1",
  "compression": "gzip",
  "bundleSize": 125000000,
  "bundleChecksum": "sha256:abc123...",
  "manifest": {
    "name": "My Backend",
    "version": "1.0.0",
    "apps": ["./app1", "./app2"],
    "platform": "linux-x64",
    "createdAt": "2024-01-15T10:30:00Z"
  },
  "opsVersion": "1.5.0",
  "createdAt": "2024-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Header format version |
| `format` | string | Always `selfhost-v1` |
| `compression` | string | Compression algorithm (`gzip`, `zstd`) |
| `bundleSize` | int64 | Uncompressed bundle size in bytes |
| `bundleChecksum` | string | SHA256 checksum of compressed bundle |
| `manifest` | object | Embedded manifest from convex-bundler |
| `opsVersion` | string | Version of embedded convex-backend-ops |
| `createdAt` | string | ISO 8601 timestamp of creation |

---

## Creating Self-Host Bundles

### Using convex-bundler

```bash
# Create standard bundle first
convex-bundler \
  --app ./convex \
  --output ./bundle \
  --name "My Backend" \
  --version "1.0.0"

# Package into self-extracting executable
convex-bundler selfhost \
  --bundle ./bundle \
  --ops-binary ./convex-backend-ops \
  --output ./my-backend-selfhost \
  --platform linux-x64
```

### Flags

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--bundle` | `-b` | Path to convex-bundler output directory | Yes |
| `--ops-binary` | `-o` | Path to convex-backend-ops binary | Yes |
| `--output` | | Output path for self-extracting executable | Yes |
| `--platform` | `-p` | Target platform (`linux-x64`, `linux-arm64`) | Yes |
| `--compression` | `-c` | Compression algorithm (`gzip`, `zstd`) | No (default: gzip) |

### Build Process

1. **Validate Inputs**
   - Verify bundle directory structure
   - Verify ops binary exists and is executable
   - Check platform compatibility

2. **Read Manifest**
   - Parse `manifest.json` from bundle
   - Extract version and app information

3. **Compress Bundle**
   - Create tar archive of bundle directory
   - Compress with specified algorithm
   - Calculate SHA256 checksum

4. **Create Header**
   - Build JSON header with all metadata
   - Length-prefix the header (4 bytes, big-endian)

5. **Assemble Executable**
   - Copy ops binary as base
   - Append start marker
   - Append length-prefixed header
   - Append compressed bundle
   - Append end marker
   - Append footer with offset

6. **Finalize**
   - Set executable permissions
   - Verify by reading back header

---

## Commands

The self-extracting executable inherits all commands from convex-backend-ops, plus additional self-host specific commands.

### Inherited Commands

All standard convex-backend-ops commands work with the embedded bundle:

```bash
# Install from embedded bundle
sudo ./my-backend-selfhost install

# Status, upgrade, rollback, etc.
sudo ./my-backend-selfhost status
sudo ./my-backend-selfhost upgrade --bundle /path/to/new-bundle
sudo ./my-backend-selfhost rollback
sudo ./my-backend-selfhost uninstall
```

### `install` (Modified)

When no `--bundle` flag is provided, extracts and uses the embedded bundle.

```bash
# Install from embedded bundle (no --bundle needed)
sudo ./my-backend-selfhost install

# Or explicitly use external bundle
sudo ./my-backend-selfhost install --bundle /path/to/bundle
```

**Implementation:**

1. Check if `--bundle` flag is provided
2. If not provided:
   - Extract embedded bundle to temp directory
   - Set bundle path to temp directory
3. Proceed with standard install flow
4. Clean up temp directory after install

### `extract`

Extracts the embedded bundle to a directory without installing.

```bash
./my-backend-selfhost extract --output /path/to/output
```

**Flags:**

| Flag | Short | Description | Required |
|------|-------|-------------|----------|
| `--output` | `-o` | Output directory for extracted bundle | Yes |

**Implementation:**

1. Read footer to find bundle offset
2. Seek to bundle start marker
3. Read and validate header
4. Decompress bundle to output directory
5. Verify checksum

### `info`

Displays information about the embedded bundle without extracting.

```bash
./my-backend-selfhost info
```

**Output:**

```
Convex Self-Host Bundle
=======================

Ops Version:    1.5.0
Bundle Name:    My Backend
Bundle Version: 1.0.0
Platform:       linux-x64
Created:        2024-01-15 10:30:00 UTC

Bundled Apps:
  - ./app1
  - ./app2

Bundle Size:    125 MB (compressed: 45 MB)
Checksum:       sha256:abc123...
```

### `verify`

Verifies the integrity of the embedded bundle.

```bash
./my-backend-selfhost verify
```

**Implementation:**

1. Read footer and header
2. Calculate SHA256 of compressed bundle section
3. Compare with stored checksum
4. Report pass/fail

**Output:**

```
✓ Bundle integrity verified
  Checksum: sha256:abc123... (matched)
```

---

## Runtime Behavior

### Self-Detection

The executable detects whether it contains an embedded bundle by:

1. Opening itself for reading
2. Reading last 8 bytes (footer) to get offset
3. Seeking to offset and checking for start marker
4. If marker found → self-host mode
5. If marker not found → standard ops mode

```go
func detectSelfHostMode() (bool, int64) {
    exe, _ := os.Executable()
    f, _ := os.Open(exe)
    defer f.Close()
    
    // Read footer (last 8 bytes)
    f.Seek(-8, io.SeekEnd)
    var offset int64
    binary.Read(f, binary.LittleEndian, &offset)
    
    // Check for magic marker
    f.Seek(offset, io.SeekStart)
    marker := make([]byte, 20)
    f.Read(marker)
    
    return string(marker) == "CONVEX_BUNDLE_START", offset
}
```

### Temporary Extraction

When installing from embedded bundle:

```go
func extractEmbeddedBundle() (string, error) {
    tempDir, _ := os.MkdirTemp("", "convex-bundle-*")
    
    exe, _ := os.Executable()
    f, _ := os.Open(exe)
    defer f.Close()
    
    // Navigate to compressed data
    offset := readFooter(f)
    f.Seek(offset + 20, io.SeekStart)  // Skip marker
    header := readHeader(f)
    
    // Decompress to temp directory
    gzReader, _ := gzip.NewReader(f)
    tarReader := tar.NewReader(gzReader)
    extractTar(tarReader, tempDir)
    
    return tempDir, nil
}
```

### Cleanup

Temporary extracted bundles are cleaned up:
- After successful install
- On install failure
- Using defer to ensure cleanup

---

## Cross-Platform Considerations

### Supported Platforms

| Platform | Architecture | Binary Format |
|----------|--------------|---------------|
| Linux | x86_64 | ELF |
| Linux | arm64 | ELF |

### Platform Detection

The embedded bundle's `platform` field must match the host:

```go
func checkPlatformCompatibility(manifest *Manifest) error {
    hostPlatform := runtime.GOOS + "-" + runtime.GOARCH
    
    // Normalize architecture names
    platformMap := map[string]string{
        "linux-amd64": "linux-x64",
        "linux-arm64": "linux-arm64",
    }
    
    normalized := platformMap[hostPlatform]
    if manifest.Platform != normalized {
        return fmt.Errorf(
            "platform mismatch: bundle is for %s, host is %s",
            manifest.Platform, normalized,
        )
    }
    return nil
}
```

---

## Security Considerations

### Checksum Verification

- Bundle integrity is verified using SHA256
- Checksum is stored in header, computed over compressed payload
- Verification runs before any extraction

### Executable Permissions

- Self-host executable should be distributed with `0755` permissions
- Extracted backend binary inherits appropriate permissions

### Credential Handling

- Credentials are extracted to `/etc/convex/` with `0600` permissions
- Admin key and instance secret are stored separately
- No credentials are logged or displayed (except admin key on first install)

---

## Size Optimization

### Compression Options

| Algorithm | Ratio | Speed | Recommended For |
|-----------|-------|-------|-----------------|
| gzip | ~65% | Fast | General use |
| zstd | ~55% | Faster | Large bundles |

### Bundle Size Estimates

| Component | Typical Size |
|-----------|--------------|
| convex-backend-ops | ~10 MB |
| convex-local-backend | ~50 MB |
| convex.db (empty) | ~100 KB |
| convex.db (with apps) | 1-50 MB |
| **Total (compressed)** | **25-60 MB** |

---

## Example Workflows

### Building and Distributing

```bash
# 1. Create Convex apps
cd my-convex-project
npx convex deploy

# 2. Bundle with convex-bundler
convex-bundler \
  --app ./convex \
  --backend-version 0.1.0 \
  --output ./bundle \
  --name "My App" \
  --version "1.0.0"

# 3. Create self-extracting executable
convex-bundler selfhost \
  --bundle ./bundle \
  --ops-binary ./convex-backend-ops-linux-x64 \
  --output ./my-app-installer \
  --platform linux-x64

# 4. Distribute single file
scp ./my-app-installer user@server:/tmp/
```

### Deploying on Target Server

```bash
# 1. Make executable
chmod +x /tmp/my-app-installer

# 2. Verify integrity
/tmp/my-app-installer verify

# 3. View bundle info
/tmp/my-app-installer info

# 4. Install
sudo /tmp/my-app-installer install

# 5. Check status
sudo /tmp/my-app-installer status
```

### Extracting Without Installing

```bash
# Extract bundle for inspection or manual install
./my-app-installer extract --output ./extracted-bundle

# View extracted contents
ls -la ./extracted-bundle/
# backend  convex.db  credentials.json  manifest.json  storage/

# Use with standard ops tool
sudo convex-backend-ops install --bundle ./extracted-bundle
```

---

## Error Handling

### Common Errors

| Error | Cause | Resolution |
|-------|-------|------------|
| `bundle checksum mismatch` | Corrupted download | Re-download the file |
| `platform mismatch` | Wrong architecture | Download correct platform build |
| `no embedded bundle found` | Using standard ops binary | Use self-host build or provide --bundle |
| `extraction failed` | Disk full or permissions | Check disk space and permissions |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Bundle verification failed |
| 4 | Platform mismatch |
| 5 | Extraction failed |
| 6 | Installation failed |

---

## Implementation Notes

### Go Embed Consideration

While Go's `//go:embed` directive could embed bundles at compile time, this approach is **not used** because:

1. Bundle content varies per deployment
2. Would require recompiling for each bundle
3. Self-appending archive is more flexible

### File Size Limits

- Maximum bundle size: 2 GB (practical limit)
- Footer uses int64 for offset, supporting files up to 8 EB theoretically

### Atomicity

Bundle extraction uses temporary directories and atomic moves to prevent partial installations.

---

## Testing

### Unit Tests

```go
func TestSelfHostDetection(t *testing.T) {
    // Test with regular binary
    isSelfHost, _ := detectSelfHostMode()
    assert.False(t, isSelfHost)
}

func TestHeaderParsing(t *testing.T) {
    header := parseHeader(sampleHeaderBytes)
    assert.Equal(t, "1.0.0", header.Version)
    assert.Equal(t, "selfhost-v1", header.Format)
}

func TestChecksumVerification(t *testing.T) {
    err := verifyBundleChecksum(testBundle)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
func TestFullExtractAndInstall(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Create self-host bundle
    selfHostPath := createTestSelfHostBundle(t)
    
    // Run in container
    container := startSystemdContainer(t)
    defer container.Terminate(ctx)
    
    // Copy and execute
    container.CopyFileToContainer(ctx, selfHostPath, "/tmp/installer", 0755)
    
    // Verify
    exitCode, output, _ := container.Exec(ctx, []string{"/tmp/installer", "verify"})
    assert.Equal(t, 0, exitCode)
    assert.Contains(t, output, "verified")
    
    // Install
    exitCode, _, _ = container.Exec(ctx, []string{"/tmp/installer", "install", "--yes"})
    assert.Equal(t, 0, exitCode)
    
    // Check service
    exitCode, _, _ = container.Exec(ctx, []string{"systemctl", "is-active", "convex-backend"})
    assert.Equal(t, 0, exitCode)
}
```

---

## Requirements

### Build Machine

- Go 1.21+
- convex-bundler
- Target platform binaries of convex-backend-ops

### Target Machine

- Linux (Ubuntu 22.04+ recommended)
- systemd
- No other dependencies required!

---

## License

Apache-2.0 - See [LICENSE](LICENSE) for details.
