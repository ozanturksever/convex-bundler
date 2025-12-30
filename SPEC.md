# Convex Bundler

A Go CLI tool that bundles Convex apps and a pre-provided backend binary into a portable package.
Source code is available at [github.com/ozanturksever/convex-bundler](https://github.com/ozanturksever/convex-bundler).

## Features

- **Version Detection**: Detects versions from CLI override, git tags, or package.json
- **Pre-deployment**: Bundles apps using `convex deploy` in a respective Docker container (orchestrated via `testcontainers-go`), creating a ready-to-use database
- **Credential Generation**: Uses `github.com/ozanturksever/convex-admin-key` to generate secure admin keys and instance secrets
- **Portable Bundle**: Creates a standalone directory/archive containing the backend and pre-initialized data

## Installation

```bash
go build -o convex-bundler .
```

### Downloading convex-local-backend

The bundler requires the `convex-local-backend` binary for pre-deployment. Download it using:

```bash
# Automatically detect platform and download
./scripts/download-backend.sh

# Or specify platform explicitly
./scripts/download-backend.sh darwin-arm64  # macOS Apple Silicon
./scripts/download-backend.sh darwin-x64    # macOS Intel
./scripts/download-backend.sh linux-x64     # Linux x86_64
./scripts/download-backend.sh linux-arm64   # Linux ARM64
```

## Usage

```bash
./convex-bundler \
  --app ./path/to/app1 \
  --app ./path/to/app2 \
  --output ./output/bundle \
  --backend-binary ./bin/convex-local-backend \
  --name "My Backend" \
  --version 1.0.0 \
  --platform linux-x64
```

### Quick Test

```bash
# Download backend binary
./scripts/download-backend.sh

# Run bundler with pre-deployment
./convex-bundler \
  --app ./testdata/sample-app \
  --output ./output/bundle \
  --backend-binary ./bin/convex-local-backend \
  --name "Test Backend"
```

### CLI Options

| Option | Short | Description | Required |
|--------|-------|-------------|----------|
| `--app` | | Path to Convex app directory (can be specified multiple times) | Yes |
| `--output` | `-o` | Output path for the bundle directory | Yes |
| `--backend-binary` | | Path to the convex-local-backend binary | Yes |
| `--name` | | Display name (default: "Convex Backend") | No |
| `--version` | | Version override (semver) | No |
| `--platform` | | Target platform: linux-x64, linux-arm64 (default: linux-x64) | No |
| `--docker-image` | | Docker image for pre-deployment (default: convex-predeploy:latest) | No |

## Bundle Contents

The generated bundle contains:

- `backend` - The convex-local-backend binary
- `convex.db` - The pre-initialized database with your apps
- `storage/` - Directory for file storage
- `manifest.json` - Metadata about the bundle (apps, version, etc.)
- `credentials.json` - Admin credentials for the backend

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with short flag (skips container tests)
go test -short ./...
```

## Docker Image

The bundler uses a custom Docker image (`convex-predeploy`) that has all dependencies pre-installed for faster pre-deployment. To build the image:

```bash
cd docker/convex-predeploy
./build.sh

# Or push to a registry:
REGISTRY=ghcr.io/your-org PUSH=true ./build.sh
```

The image includes:
- Node.js 20
- curl, unzip
- Convex CLI (npm package)
- convex-local-backend binary (for both amd64 and arm64)

If the custom image is not available, the bundler falls back to `node:20-slim` and downloads dependencies at runtime.

## Architecture

```
.
├── main.go                 # Main entry point
├── pkg/
│   ├── version/           # Version detection
│   ├── manifest/          # Manifest generation
│   ├── predeploy/         # Pre-deployment logic
│   └── cli/               # CLI parsing
├── docker/
│   └── convex-predeploy/  # Docker image for pre-deployment
├── testdata/              # Test fixtures
└── SPEC.md                # This documentation
```

## License

APACHE 2.0

