#!/bin/bash
# test-selfhost-e2e.sh
# Ad-hoc script to test selfhost bundle creation end-to-end
#
# This script:
# 1. Builds convex-bundler
# 2. Builds convex-backend-ops
# 3. Creates a test bundle using sample data
# 4. Creates a selfhost executable
# 5. Verifies the selfhost executable
# 6. Extracts and validates the bundle

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLER_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$BUNDLER_DIR/.." && pwd)"

# Configuration
OPS_DIR="$PROJECT_ROOT/convex-backend-ops"
TEST_DIR="$BUNDLER_DIR/test-selfhost-output"
# Selfhost bundles only support Linux platforms
# Use linux-x64 as default for testing (bundle creation/extraction still works)
if [[ "$(uname -m)" == "arm64" ]] || [[ "$(uname -m)" == "aarch64" ]]; then
    PLATFORM="${PLATFORM:-linux-arm64}"
else
    PLATFORM="${PLATFORM:-linux-x64}"
fi

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  Selfhost Bundle E2E Test${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""
echo -e "Platform: ${YELLOW}$PLATFORM${NC}"
echo -e "Test directory: ${YELLOW}$TEST_DIR${NC}"
echo ""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    rm -rf "$TEST_DIR"
}

# Trap to cleanup on exit (comment out to inspect results)
# trap cleanup EXIT

# Step 0: Clean up previous test
echo -e "${BLUE}Step 0: Cleaning up previous test output...${NC}"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"
echo -e "${GREEN}✓ Clean${NC}\n"

# Step 1: Build convex-bundler
echo -e "${BLUE}Step 1: Building convex-bundler...${NC}"
cd "$BUNDLER_DIR"
go build -o "$TEST_DIR/convex-bundler" .
echo -e "${GREEN}✓ Built convex-bundler${NC}\n"

# Step 2: Build convex-backend-ops
echo -e "${BLUE}Step 2: Building convex-backend-ops...${NC}"
cd "$OPS_DIR"
go build -o "$TEST_DIR/convex-backend-ops" .
echo -e "${GREEN}✓ Built convex-backend-ops${NC}\n"

# Step 3: Create a mock bundle directory
echo -e "${BLUE}Step 3: Creating mock bundle...${NC}"
BUNDLE_DIR="$TEST_DIR/bundle"
mkdir -p "$BUNDLE_DIR/storage/nested"

# Create manifest.json
cat > "$BUNDLE_DIR/manifest.json" << EOF
{
  "name": "E2E Test Backend",
  "version": "1.0.0",
  "apps": ["./test-app"],
  "platform": "$PLATFORM",
  "createdAt": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF

# Create mock backend binary
echo '#!/bin/bash
echo "Mock Convex Backend v0.1.0"' > "$BUNDLE_DIR/backend"
chmod +x "$BUNDLE_DIR/backend"

# Create mock database
echo "SQLite format 3 - mock database content for testing" > "$BUNDLE_DIR/convex.db"

# Create credentials.json
cat > "$BUNDLE_DIR/credentials.json" << EOF
{
  "adminKey": "convex_admin_test_key_12345",
  "instanceSecret": "$(openssl rand -hex 32 2>/dev/null || echo 'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')"
}
EOF

# Create some test files in storage
echo "Test storage file content" > "$BUNDLE_DIR/storage/test-file.txt"
echo "Nested content" > "$BUNDLE_DIR/storage/nested/deep-file.txt"

echo -e "${GREEN}✓ Created mock bundle at $BUNDLE_DIR${NC}"
echo "  Contents:"
ls -la "$BUNDLE_DIR"
echo ""

# Step 4: Create selfhost executable
echo -e "${BLUE}Step 4: Creating selfhost executable...${NC}"
SELFHOST_PATH="$TEST_DIR/my-backend-selfhost"

"$TEST_DIR/convex-bundler" selfhost \
    --bundle "$BUNDLE_DIR" \
    --ops-binary "$TEST_DIR/convex-backend-ops" \
    --output "$SELFHOST_PATH" \
    --platform "$PLATFORM" \
    --compression gzip \
    --ops-version "1.0.0-test"

if [ -f "$SELFHOST_PATH" ]; then
    echo -e "${GREEN}✓ Created selfhost executable${NC}"
    echo "  Size: $(du -h "$SELFHOST_PATH" | cut -f1)"
    echo ""
else
    echo -e "${RED}✗ Failed to create selfhost executable${NC}"
    exit 1
fi

# Step 5: Test 'info' command
echo -e "${BLUE}Step 5: Testing 'info' command...${NC}"
"$SELFHOST_PATH" info
echo -e "${GREEN}✓ Info command works${NC}\n"

# Step 6: Test 'verify' command
echo -e "${BLUE}Step 6: Testing 'verify' command...${NC}"
"$SELFHOST_PATH" verify
echo -e "${GREEN}✓ Verify command works${NC}\n"

# Step 7: Test 'extract' command
echo -e "${BLUE}Step 7: Testing 'extract' command...${NC}"
EXTRACT_DIR="$TEST_DIR/extracted"
"$SELFHOST_PATH" extract --output "$EXTRACT_DIR"

echo "  Extracted contents:"
ls -la "$EXTRACT_DIR"
echo ""

# Verify extracted files
if [ -f "$EXTRACT_DIR/backend" ] && \
   [ -f "$EXTRACT_DIR/convex.db" ] && \
   [ -f "$EXTRACT_DIR/manifest.json" ] && \
   [ -f "$EXTRACT_DIR/credentials.json" ] && \
   [ -d "$EXTRACT_DIR/storage" ]; then
    echo -e "${GREEN}✓ All expected files extracted${NC}\n"
else
    echo -e "${RED}✗ Missing expected files in extracted bundle${NC}"
    exit 1
fi

# Step 8: Compare original and extracted
echo -e "${BLUE}Step 8: Comparing original and extracted bundles...${NC}"

# Compare manifest
if diff -q "$BUNDLE_DIR/manifest.json" "$EXTRACT_DIR/manifest.json" > /dev/null; then
    echo -e "  ${GREEN}✓ manifest.json matches${NC}"
else
    echo -e "  ${RED}✗ manifest.json differs${NC}"
    exit 1
fi

# Compare credentials
if diff -q "$BUNDLE_DIR/credentials.json" "$EXTRACT_DIR/credentials.json" > /dev/null; then
    echo -e "  ${GREEN}✓ credentials.json matches${NC}"
else
    echo -e "  ${RED}✗ credentials.json differs${NC}"
    exit 1
fi

# Compare convex.db
if diff -q "$BUNDLE_DIR/convex.db" "$EXTRACT_DIR/convex.db" > /dev/null; then
    echo -e "  ${GREEN}✓ convex.db matches${NC}"
else
    echo -e "  ${RED}✗ convex.db differs${NC}"
    exit 1
fi

# Compare storage files
if diff -q "$BUNDLE_DIR/storage/test-file.txt" "$EXTRACT_DIR/storage/test-file.txt" > /dev/null; then
    echo -e "  ${GREEN}✓ storage/test-file.txt matches${NC}"
else
    echo -e "  ${RED}✗ storage/test-file.txt differs${NC}"
    exit 1
fi

if diff -q "$BUNDLE_DIR/storage/nested/deep-file.txt" "$EXTRACT_DIR/storage/nested/deep-file.txt" > /dev/null; then
    echo -e "  ${GREEN}✓ storage/nested/deep-file.txt matches${NC}"
else
    echo -e "  ${RED}✗ storage/nested/deep-file.txt differs${NC}"
    exit 1
fi

echo ""

# Step 9: Test JSON output
echo -e "${BLUE}Step 9: Testing JSON output mode...${NC}"
JSON_OUTPUT=$("$SELFHOST_PATH" info --json)
if echo "$JSON_OUTPUT" | grep -q '"isSelfHost": true'; then
    echo -e "${GREEN}✓ JSON output works${NC}"
    echo "$JSON_OUTPUT" | head -10
    echo "  ..."
else
    echo -e "${RED}✗ JSON output failed${NC}"
    exit 1
fi
echo ""

# Summary
echo -e "${BLUE}============================================${NC}"
echo -e "${GREEN}  All E2E Tests Passed! ✓${NC}"
echo -e "${BLUE}============================================${NC}"
echo ""
echo "Test artifacts location: $TEST_DIR"
echo ""
echo "You can inspect:"
echo "  - Original bundle: $BUNDLE_DIR"
echo "  - Selfhost executable: $SELFHOST_PATH"
echo "  - Extracted bundle: $EXTRACT_DIR"
echo ""
echo "To clean up: rm -rf $TEST_DIR"
