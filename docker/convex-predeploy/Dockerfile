# Multi-arch Dockerfile for Convex pre-deployment
# Supports both linux/amd64 and linux/arm64
# Build with: docker buildx build --platform linux/amd64,linux/arm64 -t convex-predeploy .

FROM node:20-slim

# Build arguments for multi-arch support
ARG TARGETARCH
ARG BACKEND_RELEASE_TAG=precompiled-2025-12-12-73e805a

# Install system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        unzip \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install convex CLI globally
RUN npm install -g convex

# Download the appropriate convex-local-backend binary based on architecture
RUN set -ex; \
    case "${TARGETARCH}" in \
        amd64) PLATFORM="x86_64-unknown-linux-gnu" ;; \
        arm64) PLATFORM="aarch64-unknown-linux-gnu" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac; \
    curl -L -o /tmp/convex-local-backend.zip \
        "https://github.com/get-convex/convex-backend/releases/download/${BACKEND_RELEASE_TAG}/convex-local-backend-${PLATFORM}.zip" && \
    unzip -o /tmp/convex-local-backend.zip -d /usr/local/bin && \
    chmod +x /usr/local/bin/convex-local-backend && \
    rm /tmp/convex-local-backend.zip

# Verify the binary works
RUN /usr/local/bin/convex-local-backend --version

# Set working directory
WORKDIR /workspace

# Default command
CMD ["sleep", "infinity"]
