#!/bin/bash

# Build script for convex-predeploy Docker image
# This script builds a multi-arch image for both amd64 and arm64

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Default values
IMAGE_NAME="${IMAGE_NAME:-convex-predeploy}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGISTRY:-}"
PUSH="${PUSH:-false}"

# Full image name
if [ -n "$REGISTRY" ]; then
    FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
else
    FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
fi

echo "Building convex-predeploy Docker image..."
echo "  Image: ${FULL_IMAGE}"
echo "  Push: ${PUSH}"

# Check if buildx is available
if ! docker buildx version > /dev/null 2>&1; then
    echo "Error: docker buildx is required for multi-arch builds"
    echo "Please install Docker Buildx: https://docs.docker.com/buildx/working-with-buildx/"
    exit 1
fi

# Create builder if it doesn't exist
if ! docker buildx inspect convex-builder > /dev/null 2>&1; then
    echo "Creating buildx builder..."
    docker buildx create --name convex-builder --use
fi

# Use the builder
docker buildx use convex-builder

# Build command
BUILD_CMD="docker buildx build --platform linux/amd64,linux/arm64 -t ${FULL_IMAGE}"

if [ "$PUSH" = "true" ]; then
    BUILD_CMD="${BUILD_CMD} --push"
else
    # Load single-arch image for local testing
    echo "Building for local architecture only (use PUSH=true for multi-arch)..."
    BUILD_CMD="docker buildx build --load -t ${FULL_IMAGE}"
fi

# Run the build
echo "Running: ${BUILD_CMD} ."
${BUILD_CMD} .

echo ""
echo "Build complete!"
if [ "$PUSH" = "true" ]; then
    echo "Image pushed to: ${FULL_IMAGE}"
else
    echo "Image available locally: ${FULL_IMAGE}"
    echo ""
    echo "To push multi-arch image to a registry:"
    echo "  REGISTRY=ghcr.io/your-org PUSH=true ./build.sh"
fi
