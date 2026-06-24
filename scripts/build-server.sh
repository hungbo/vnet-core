#!/bin/bash
set -e

VERSION=${1:-dev}
echo "=== Building VNET Server v$VERSION ==="
echo "-> Building Admin UI..."
cd "$(dirname "$0")/../admin"
npm ci 2>/dev/null
npm run build
mkdir -p ../backend/cmd/server/embed
cp -r dist/* ../backend/cmd/server/embed/

echo "-> Building Server..."
cd ../backend
go build -ldflags="-s -w -X main.version=$VERSION" -o vnet-server ./cmd/server

echo "=== Done: backend/vnet-server ==="
sha256sum vnet-server
ls -lh vnet-server
