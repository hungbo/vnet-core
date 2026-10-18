#!/bin/bash
set -e

VERSION=${VERSION:-"0.1.0"}
ARCH=${ARCH:-"amd64"}
OUTPUT_DIR=${OUTPUT_DIR:-"dist"}
SUFFIX=""
if [ "$ARCH" = "arm64" ]; then
  SUFFIX="-arm64"
fi

echo "=== Building VNET Client v$VERSION (${ARCH}) ==="

mkdir -p "$OUTPUT_DIR"

echo "--- Building VNET Client (${ARCH}) ---"
cd src
wails build -platform "windows/$ARCH" -ldflags "-X 'main.Version=$VERSION' -s -w" \
  -o "../$OUTPUT_DIR/vnet-client${SUFFIX}.exe"

echo ""
echo "=== Build Complete ==="
echo "Output: $OUTPUT_DIR/ ($ARCH)"
ls -lh "$OUTPUT_DIR/"
