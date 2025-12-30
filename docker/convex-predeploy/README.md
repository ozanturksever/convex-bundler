# Convex Pre-deployment Docker Image

This Docker image contains all dependencies needed for Convex pre-deployment, significantly speeding up the bundling process by eliminating the need to download and install tools at runtime.

## Pre-installed Dependencies

- **Node.js 20** (slim base image)
- **curl** and **unzip** (system tools)
- **Convex CLI** (installed globally via npm)
- **convex-local-backend** (pre-downloaded for the target architecture)

## Supported Architectures

The image supports both:
- `linux/amd64` (x86_64)
- `linux/arm64` (aarch64, Apple Silicon)

## Building the Image

### Local Build (single architecture)

```bash
cd docker/convex-predeploy
./build.sh
```

This creates `convex-predeploy:latest` for your local architecture.

### Multi-arch Build & Push

```bash
cd docker/convex-predeploy
REGISTRY=ghcr.io/your-org PUSH=true ./build.sh
```

This builds for both amd64 and arm64 and pushes to the registry.

### Custom Tag

```bash
IMAGE_TAG=v1.0.0 REGISTRY=ghcr.io/your-org PUSH=true ./build.sh
```

## Using with convex-bundler

The `convex-bundler` CLI automatically uses this image when available. You can also specify a custom image:

```bash
# Uses the default convex-predeploy:latest image
./convex-bundler --app ./my-app --output ./bundle --backend-binary ./bin/convex-local-backend

# Or specify a custom image (falls back to downloading dependencies at runtime)
./convex-bundler --app ./my-app --output ./bundle --backend-binary ./bin/convex-local-backend --docker-image node:20-slim
```

## Performance Comparison

| Approach | Time |
|----------|------|
| `convex-predeploy:latest` | ~10-15 seconds |
| `node:20-slim` (downloads at runtime) | ~60-90 seconds |

The pre-built image saves significant time by having all dependencies pre-installed.

## Updating the Backend Version

To update the convex-local-backend version, modify the `BACKEND_RELEASE_TAG` build argument:

```bash
docker buildx build \
  --build-arg BACKEND_RELEASE_TAG=precompiled-2025-01-15-xxxxx \
  --platform linux/amd64,linux/arm64 \
  -t convex-predeploy:latest .
```

Or update the default value in the Dockerfile.
