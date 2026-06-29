#!/bin/bash
set -e

VERSION=${1:-dev}
echo "=== Building VNET Desktop (Agent + Client) v$VERSION ==="

cd "$(dirname "$0")/../client"

echo "-> Building Agent..."
GOOS=windows GOARCH=amd64 go build -buildvcs=false \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/vnet-agent.exe ./cmd/agent/

echo "-> Building Client..."
cd cmd/client
npm install --silent 2>/dev/null
npm run build --silent 2>/dev/null
cd ../..
go build -buildvcs=false \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/vnet-client.exe ./cmd/client/

echo "-> Packing installer..."
cp installer/install.bat dist/

cd dist
zip -r "../vnet-client-windows-${VERSION}.zip" ./*

echo "=== Done: vnet-client-windows-${VERSION}.zip ==="
ls -lh "../vnet-client-windows-${VERSION}.zip"
